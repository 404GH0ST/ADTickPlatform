package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

type testSubmissionGameCoreClient struct {
	submit  []apigateway.SubmissionVerdictAlias
	attacks apigateway.AttackFeedPage
	err     error
}

func (c testSubmissionGameCoreClient) SubmitFlags(_ context.Context, _ int, _ []string) ([]apigateway.SubmissionVerdictAlias, error) {
	if c.err != nil {
		return nil, c.err
	}
	if len(c.submit) == 0 {
		return []apigateway.SubmissionVerdictAlias{{Flag: "FLAGv1.demo", Verdict: "flag is correct."}}, nil
	}
	return c.submit, nil
}

func (c testSubmissionGameCoreClient) AttackFeed(_ context.Context, _ apigateway.AttackFeedQuery) (apigateway.AttackFeedPage, error) {
	if c.err != nil {
		return apigateway.AttackFeedPage{}, c.err
	}
	if len(c.attacks.Items) == 0 {
		return apigateway.AttackFeedPage{
			Items: []apigateway.AttackEventAlias{{
				ID:       "atk-1",
				Attacker: "Team Alpha",
				Victim:   "Team Delta",
				Service:  "banking",
				Tick:     4,
				Verdict:  "first valid submission accepted",
			}},
			Limit:      12,
			Offset:     0,
			TotalCount: 1,
		}, nil
	}
	return c.attacks, nil
}

func TestSubmissionServiceSubmit(t *testing.T) {
	server := newSubmissionServiceServer("dev-admin-token", testSubmissionGameCoreClient{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "submission-service", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/submissions/submit", bytes.NewBufferString(`{"team_id":101,"flags":["FLAGv1.demo"]}`))
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string                              `json:"status"`
		Data   []apigateway.SubmissionVerdictAlias `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode submit response: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Verdict != "flag is correct." {
		t.Fatalf("unexpected payload %+v", payload.Data)
	}
}

func TestSubmissionServiceAttackFeed(t *testing.T) {
	server := newSubmissionServiceServer("dev-admin-token", testSubmissionGameCoreClient{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "submission-service", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/internal/v1/submissions/attacks?service=bank", nil)
	request.Header.Set("Authorization", "Bearer dev-admin-token")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload struct {
		Status string                    `json:"status"`
		Data   apigateway.AttackFeedPage `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode attack feed response: %v", err)
	}
	if len(payload.Data.Items) != 1 || payload.Data.Items[0].Service != "banking" {
		t.Fatalf("unexpected attack feed %+v", payload.Data)
	}
}

func TestSubmissionServiceRequiresAdminAuth(t *testing.T) {
	server := newSubmissionServiceServer("dev-admin-token", testSubmissionGameCoreClient{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "submission-service", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	request := httptest.NewRequest(http.MethodPost, "/internal/v1/submissions/submit", bytes.NewBufferString(`{"team_id":101,"flags":["FLAGv1.demo"]}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}
