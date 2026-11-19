package apigateway

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"adplatform/internal/platform/httpapi"
)

func TestNormalizeTeamInstitution(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{raw: "", want: "", ok: true},
		{raw: "  Institut   Teknologi\tBandung  ", want: "Institut Teknologi Bandung", ok: true},
		{raw: strings.Repeat("é", maxTeamInstitutionLength), want: strings.Repeat("é", maxTeamInstitutionLength), ok: true},
		{raw: strings.Repeat("a", maxTeamInstitutionLength+1), ok: false},
		{raw: "Uni\x00versity", ok: false},
	}
	for _, tc := range cases {
		got, ok := normalizeTeamInstitution(tc.raw)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("normalizeTeamInstitution(%q) = %q, %v; want %q, %v", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
	if got := storedTeamInstitution(strings.Repeat("b", maxTeamInstitutionLength+5)); len([]rune(got)) != maxTeamInstitutionLength {
		t.Fatalf("storedTeamInstitution must clamp to %d runes, got %d", maxTeamInstitutionLength, len([]rune(got)))
	}
}

func newInstitutionTestServer(t *testing.T) (*http.ServeMux, *memoryStore) {
	t.Helper()
	store := NewMemoryStore(101)
	memory, ok := store.(*memoryStore)
	if !ok {
		t.Fatalf("expected *memoryStore, got %T", store)
	}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, storeBackedControllerClient{store: store}, noopWireGuardClient{}).RegisterRoutes(mux)
	return mux, memory
}

func serveInstitutionRequest(t *testing.T, mux *http.ServeMux, method, path, body string, admin bool) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = &bytes.Buffer{}
	}
	request := httptest.NewRequest(method, path, reader)
	if admin {
		setTestAdminAuthHeader(request)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func TestAdminTeamInstitutionCreateUpdateAndKeep(t *testing.T) {
	mux, _ := newInstitutionTestServer(t)

	created := serveInstitutionRequest(t, mux, http.MethodPost, "/api/v2/admin/teams",
		`{"name":"Fresh Team","contact_email":"fresh@example.com","institution":"  Universitas   Indonesia "}`, true)
	if created.Code != http.StatusOK {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	team := decodeCompat[adminTeam](t, created.Body.Bytes())
	if team.Institution != "Universitas Indonesia" {
		t.Fatalf("create institution = %q", team.Institution)
	}

	// Omitting the field keeps the stored institution.
	kept := serveInstitutionRequest(t, mux, http.MethodPut, "/api/v2/admin/teams/101",
		`{"name":"Team Alpha","contact_email":"team-alpha@example.com","institution":"Institut Teknologi Bandung"}`, true)
	if kept.Code != http.StatusOK {
		t.Fatalf("update: %d %s", kept.Code, kept.Body.String())
	}
	renamed := serveInstitutionRequest(t, mux, http.MethodPut, "/api/v2/admin/teams/101",
		`{"name":"Team Alpha","contact_email":"team-alpha@example.com"}`, true)
	if got := decodeCompat[adminTeam](t, renamed.Body.Bytes()).Institution; got != "Institut Teknologi Bandung" {
		t.Fatalf("update without institution must keep it, got %q", got)
	}

	// An empty string clears it.
	cleared := serveInstitutionRequest(t, mux, http.MethodPut, "/api/v2/admin/teams/101",
		`{"name":"Team Alpha","contact_email":"team-alpha@example.com","institution":""}`, true)
	if got := decodeCompat[adminTeam](t, cleared.Body.Bytes()).Institution; got != "" {
		t.Fatalf("empty institution must clear it, got %q", got)
	}

	tooLong := `{"name":"Team Alpha","contact_email":"team-alpha@example.com","institution":"` + strings.Repeat("x", maxTeamInstitutionLength+1) + `"}`
	for _, request := range []struct{ method, path string }{
		{http.MethodPut, "/api/v2/admin/teams/101"},
		{http.MethodPost, "/api/v2/admin/teams"},
	} {
		body := tooLong
		if request.method == http.MethodPost {
			body = strings.Replace(tooLong, "team-alpha@example.com", "other@example.com", 1)
			body = strings.Replace(body, `"Team Alpha"`, `"Other Team"`, 1)
		}
		if response := serveInstitutionRequest(t, mux, request.method, request.path, body, true); response.Code != http.StatusBadRequest {
			t.Fatalf("%s %s with a long institution: got %d, want 400", request.method, request.path, response.Code)
		}
	}
}

func TestScoreboardAndAttackFeedCarryTeamInstitutions(t *testing.T) {
	mux, store := newInstitutionTestServer(t)
	if _, err := store.UpdateAdminTeam(context.Background(), 101, adminUpdateTeamRequest{
		Name: "Team Alpha", ContactEmail: "team-alpha@example.com", Institution: ptr("Institut Teknologi Bandung"),
	}); err != nil {
		t.Fatalf("seed institution: %v", err)
	}
	store.mu.Lock()
	store.attackFeed = []attackEvent{{ID: "atk-1", Attacker: "Team Alpha", Victim: "Team Delta", Service: "web", Tick: 3, Verdict: "accepted"}}
	store.mu.Unlock()

	board := serveInstitutionRequest(t, mux, http.MethodGet, "/api/v2/scoreboard", "", false)
	rows := decodeCompat[[]scoreRow](t, board.Body.Bytes())
	byTeam := map[string]string{}
	for _, row := range rows {
		byTeam[row.Team] = row.Institution
	}
	if byTeam["Team Alpha"] != "Institut Teknologi Bandung" || byTeam["Team Delta"] != "" {
		t.Fatalf("scoreboard institutions = %v", byTeam)
	}
	if strings.Contains(board.Body.String(), `"institution":""`) {
		t.Fatalf("empty institutions must be omitted: %s", board.Body.String())
	}

	feed := serveInstitutionRequest(t, mux, http.MethodGet, "/api/v2/attacks", "", false)
	page := decodeCompat[AttackFeedPage](t, feed.Body.Bytes())
	if len(page.Items) != 1 || page.Items[0].AttackerInstitution != "Institut Teknologi Bandung" || page.Items[0].VictimInstitution != "" {
		t.Fatalf("attack feed institutions = %+v", page.Items)
	}
}

func TestInstitutionEnrichmentIsBestEffortAndCopiesRows(t *testing.T) {
	snapshot := []scoreRow{{Rank: 1, Team: "A"}}
	enriched := withScoreInstitutions(snapshot, map[string]string{"A": "Inst"})
	if enriched[0].Institution != "Inst" || snapshot[0].Institution != "" {
		t.Fatalf("enrichment must not mutate its input: in=%+v out=%+v", snapshot, enriched)
	}
	if withScoreInstitutions(nil, nil) != nil || withAttackInstitutions(nil, nil) != nil {
		t.Fatal("nil payloads must stay nil")
	}

	server := &Server{store: failingStore{}}
	if got := server.teamInstitutionsByName(context.Background()); got != nil {
		t.Fatalf("a failing lookup must yield no institutions, got %v", got)
	}
}

func ptr[T any](value T) *T { return &value }
