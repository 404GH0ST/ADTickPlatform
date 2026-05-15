package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		return []apigateway.SubmissionVerdictAlias{{Flag: "FLAGv1.demo", Status: "accepted", Detail: "flag is correct."}}, nil
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

	var payload []apigateway.SubmissionVerdictAlias
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode submit response: %v", err)
	}
	if len(payload) != 1 || payload[0].Detail != "flag is correct." {
		t.Fatalf("unexpected payload %+v", payload)
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

	var payload apigateway.AttackFeedPage
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode attack feed response: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].Service != "banking" {
		t.Fatalf("unexpected attack feed %+v", payload)
	}
}

func TestSubmissionServiceRoutesRequireAdminAuth(t *testing.T) {
	server := newSubmissionServiceServer("dev-admin-token", testSubmissionGameCoreClient{})
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "submission-service", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/internal/v1/submissions/submit", `{"team_id":101,"flags":["FLAGv1.demo"]}`},
		{http.MethodGet, "/internal/v1/submissions/attacks?service=bank", ""},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path+" unauthenticated", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})

		t.Run(tc.method+" "+tc.path+" wrong token", func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer wrong-token")
			response := httptest.NewRecorder()

			mux.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestSubmissionServiceMetricsEndpoint(t *testing.T) {
	info := httpapi.ServiceInfo{Name: "submission-service-metrics-" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-")), Version: "test", Addr: ":0"}
	server := newSubmissionServiceServer("dev-admin-token", testSubmissionGameCoreClient{
		submit: []apigateway.SubmissionVerdictAlias{
			{Flag: "FLAGv1.ok", Status: "accepted", Detail: "flag is correct."},
			{Flag: "FLAGv1.dupe", Status: "duplicate", Detail: "flag already submitted."},
			{Flag: "FLAGv1.bad", Status: "invalid", Detail: "flag is wrong or expired."},
		},
	})
	httpapi.RegisterMetricsSource(info.Name, server)

	mux := httpapi.NewBaseMux(info)
	server.RegisterRoutes(mux)

	submitRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/submissions/submit", bytes.NewBufferString(`{"team_id":101,"flags":["FLAGv1.ok","FLAGv1.dupe","FLAGv1.bad"]}`))
	submitRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	submitResponse := httptest.NewRecorder()
	mux.ServeHTTP(submitResponse, submitRequest)
	if submitResponse.Code != http.StatusOK {
		t.Fatalf("expected submit 200, got %d", submitResponse.Code)
	}

	attacksRequest := httptest.NewRequest(http.MethodGet, "/internal/v1/submissions/attacks?service=bank", nil)
	attacksRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	attacksResponse := httptest.NewRecorder()
	mux.ServeHTTP(attacksResponse, attacksRequest)
	if attacksResponse.Code != http.StatusOK {
		t.Fatalf("expected attacks 200, got %d", attacksResponse.Code)
	}

	metricsRequest := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResponse := httptest.NewRecorder()
	mux.ServeHTTP(metricsResponse, metricsRequest)

	body := metricsResponse.Body.String()
	for _, fragment := range []string{
		"adplatform_submission_service_submit_requests_total 1",
		"adplatform_submission_service_submit_flags_total 3",
		`adplatform_submission_service_submit_verdicts_total{class="correct"} 1`,
		`adplatform_submission_service_submit_verdicts_total{class="duplicate"} 1`,
		`adplatform_submission_service_submit_verdicts_total{class="invalid"} 1`,
		"adplatform_submission_service_attack_feed_requests_total 1",
		"adplatform_submission_service_attack_feed_items_total 1",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", fragment, body)
		}
	}
}
