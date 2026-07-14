package apigateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestChangeParticipantPassword(t *testing.T) {
	store := NewMemoryStore(101)
	mux := http.NewServeMux()
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)

	loginBody := bytes.NewBufferString(`{"email":"alpha.captain@example.com","password":"alpha-secret"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v2/authenticate", loginBody)
	loginRes := httptest.NewRecorder()
	mux.ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", loginRes.Code, loginRes.Body.String())
	}
	var auth authenticateResponse
	if err := json.Unmarshal(loginRes.Body.Bytes(), &auth); err != nil {
		t.Fatalf("decode auth: %v", err)
	}

	changeBody := bytes.NewBufferString(`{"current_password":"alpha-secret","new_password":"alpha-new-pass"}`)
	changeReq := httptest.NewRequest(http.MethodPut, "/api/v2/me/password", changeBody)
	changeReq.Header.Set("Authorization", "Bearer "+auth.Token)
	changeRes := httptest.NewRecorder()
	mux.ServeHTTP(changeRes, changeReq)
	if changeRes.Code != http.StatusOK {
		t.Fatalf("password change status %d: %s", changeRes.Code, changeRes.Body.String())
	}
	var changed struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(changeRes.Body.Bytes(), &changed); err != nil || changed.Token == "" {
		t.Fatalf("decode replacement token: %v (%s)", err, changeRes.Body.String())
	}

	oldSessionReq := httptest.NewRequest(http.MethodGet, "/api/v2/session", nil)
	oldSessionReq.Header.Set("Authorization", "Bearer "+auth.Token)
	oldSessionRes := httptest.NewRecorder()
	mux.ServeHTTP(oldSessionRes, oldSessionReq)
	if oldSessionRes.Code != http.StatusForbidden {
		t.Fatalf("expected old token to be revoked, got %d: %s", oldSessionRes.Code, oldSessionRes.Body.String())
	}

	newSessionReq := httptest.NewRequest(http.MethodGet, "/api/v2/session", nil)
	newSessionReq.Header.Set("Authorization", "Bearer "+changed.Token)
	newSessionRes := httptest.NewRecorder()
	mux.ServeHTTP(newSessionRes, newSessionReq)
	if newSessionRes.Code != http.StatusOK {
		t.Fatalf("expected replacement token to work, got %d: %s", newSessionRes.Code, newSessionRes.Body.String())
	}

	if _, err := store.AuthenticatePlayer(context.Background(), "alpha.captain@example.com", "alpha-new-pass"); err != nil {
		t.Fatalf("authenticate with new password: %v", err)
	}
}

func TestChangeParticipantPasswordPreservesWhitespace(t *testing.T) {
	store := NewMemoryStore(101)
	registered, err := store.RegisterPlayer(context.Background(), participantRegisterRequest{
		DisplayName: "Whitespace Password",
		Email:       "whitespace.password@example.com",
		Password:    "  exact password  ",
	}, time.Now())
	if err != nil {
		t.Fatalf("register player: %v", err)
	}
	token, err := issueTeamJWT("dev-team-token", registered, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	mux := http.NewServeMux()
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)
	changeBody := bytes.NewBufferString(`{"current_password":"  exact password  ","new_password":"  replacement password  "}`)
	changeReq := httptest.NewRequest(http.MethodPut, "/api/v2/me/password", changeBody)
	changeReq.Header.Set("Authorization", "Bearer "+token)
	changeRes := httptest.NewRecorder()
	mux.ServeHTTP(changeRes, changeReq)

	if changeRes.Code != http.StatusOK {
		t.Fatalf("password change status %d: %s", changeRes.Code, changeRes.Body.String())
	}
	if _, err := store.AuthenticatePlayer(context.Background(), registered.Email, "  replacement password  "); err != nil {
		t.Fatalf("authenticate with exact replacement password: %v", err)
	}
	if _, err := store.AuthenticatePlayer(context.Background(), registered.Email, "replacement password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected trimmed password to fail, got %v", err)
	}
}

func TestChangeParticipantPasswordUsesIndependentClientAndUserRateLimits(t *testing.T) {
	limiter := &recordingRateLimiter{}
	mux := newTestMuxWithLimiter(limiter)
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v2/me/password",
		bytes.NewBufferString(`{"current_password":"wrong-password","new_password":"replacement-password"}`),
	)
	request.RemoteAddr = "198.51.100.24:44123"
	request.Header.Set("Authorization", testTeamBearerToken(t, 101))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected wrong current password 403, got %d: %s", response.Code, response.Body.String())
	}
	wantKeys := []string{
		"password-change:client:198.51.100.24",
		"password-change:user:1",
	}
	if !slices.Equal(limiter.keys, wantKeys) {
		t.Fatalf("expected password-change rate-limit keys %v, got %v", wantKeys, limiter.keys)
	}
}

func TestChangeParticipantPasswordRateLimitFailsClosed(t *testing.T) {
	mux := newTestMuxWithLimiter(unavailableRateLimiter{})
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v2/me/password",
		bytes.NewBufferString(`{"current_password":"alpha-secret","new_password":"replacement-password"}`),
	)
	request.RemoteAddr = "198.51.100.25:44123"
	request.Header.Set("Authorization", testTeamBearerToken(t, 101))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected unavailable limiter to reject password change with 429, got %d: %s", response.Code, response.Body.String())
	}
}

func TestConcurrentPasswordChangesAcceptCurrentPasswordOnlyOnce(t *testing.T) {
	store := NewMemoryStore(101)
	passwords := []string{"first-concurrent-password", "second-concurrent-password"}
	start := make(chan struct{})
	type result struct {
		password string
		err      error
	}
	results := make(chan result, len(passwords))
	var ready sync.WaitGroup
	ready.Add(len(passwords))
	for _, password := range passwords {
		go func() {
			ready.Done()
			<-start
			_, err := store.ChangeParticipantPassword(context.Background(), 1, "alpha-secret", password)
			results <- result{password: password, err: err}
		}()
	}
	ready.Wait()
	close(start)

	var successfulPassword string
	invalidCount := 0
	for range passwords {
		result := <-results
		switch {
		case result.err == nil:
			if successfulPassword != "" {
				t.Fatalf("multiple concurrent password changes succeeded")
			}
			successfulPassword = result.password
		case errors.Is(result.err, ErrInvalidCredentials):
			invalidCount++
		default:
			t.Fatalf("unexpected password change error: %v", result.err)
		}
	}
	if successfulPassword == "" || invalidCount != 1 {
		t.Fatalf("expected one success and one rejected stale password, success=%q rejected=%d", successfulPassword, invalidCount)
	}
	if _, err := store.AuthenticatePlayer(context.Background(), "alpha.captain@example.com", successfulPassword); err != nil {
		t.Fatalf("authenticate with winning password: %v", err)
	}
}

func TestAnnouncements(t *testing.T) {
	store := NewMemoryStore(101)
	mux := http.NewServeMux()
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)

	createBody := bytes.NewBufferString(`{"body":"Warmup in 10 minutes"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v2/admin/announcements", createBody)
	createReq.Header.Set("Authorization", "Bearer dev-admin-token")
	createRes := httptest.NewRecorder()
	mux.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusOK {
		t.Fatalf("create announcement status %d: %s", createRes.Code, createRes.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v2/announcements", nil)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list announcements status %d: %s", listRes.Code, listRes.Body.String())
	}
	var announcements []matchAnnouncement
	if err := json.Unmarshal(listRes.Body.Bytes(), &announcements); err != nil {
		t.Fatalf("decode announcements: %v", err)
	}
	if len(announcements) != 1 || announcements[0].Body != "Warmup in 10 minutes" {
		t.Fatalf("unexpected announcements: %+v", announcements)
	}
}

func TestBulkImportTeamsAtomicPerTeam(t *testing.T) {
	store := NewMemoryStore(101)
	mux := http.NewServeMux()
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)

	// First team is valid; second has invalid player mid-row after team name would create.
	body := bytes.NewBufferString(`{
		"teams":[
			{
				"name":"Team Import",
				"contact_email":"import@example.com",
				"players":[
					{"display_name":"Import Cap","email":"import.cap@example.com","password":"import-secret","role":"captain"}
				]
			},
			{
				"name":"Team Broken",
				"contact_email":"broken@example.com",
				"players":[
					{"display_name":"","email":"broken@example.com","password":"x","role":"member"}
				]
			}
		]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/admin/import/teams", body)
	req.Header.Set("Authorization", "Bearer dev-admin-token")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("bulk import status %d: %s", res.Code, res.Body.String())
	}
	var result adminBulkImportResult
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode import result: %v", err)
	}
	if result.TeamsCreated != 1 || result.PlayersCreated != 1 {
		t.Fatalf("unexpected import result: %+v", result)
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected validation error for broken team")
	}
	if _, err := store.AuthenticatePlayer(context.Background(), "import.cap@example.com", "import-secret"); err != nil {
		t.Fatalf("imported player auth: %v", err)
	}

	// All-failed import returns 422.
	failBody := bytes.NewBufferString(`{"teams":[{"name":"","contact_email":""}]}`)
	failReq := httptest.NewRequest(http.MethodPost, "/api/v2/admin/import/teams", failBody)
	failReq.Header.Set("Authorization", "Bearer dev-admin-token")
	failRes := httptest.NewRecorder()
	mux.ServeHTTP(failRes, failReq)
	if failRes.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for empty import, got %d: %s", failRes.Code, failRes.Body.String())
	}
}
