package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"adplatform/internal/services/apigateway"
)

func TestHTTPCheckerClientExecuteAndBatch(t *testing.T) {
	if _, ok := newCheckerClient("", "t").(noopCheckerClient); !ok {
		t.Fatal("blank base URL must yield the dry-run client")
	}
	type seen struct{ path, auth, ctype, body string }
	var requests []seen
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		requests = append(requests, seen{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(data)})
		if strings.HasSuffix(r.URL.Path, "-batch") {
			_, _ = w.Write([]byte(`{"challenge_id":2,"team_id":3,"phases":[{"phase":"put","status":"success"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"challenge_id":2,"team_id":3,"phase":"check","status":"success","service_state":"ok"}`))
	}))
	defer server.Close()

	client := newCheckerClient(server.URL, " runner-token ")
	result, err := client.Execute(context.Background(), apigateway.CheckerExecutionRequest{ChallengeID: 2, TeamID: 3, Phase: "check", Flag: "F"})
	if err != nil || result.ServiceState != "ok" {
		t.Fatalf("execute %+v %v", result, err)
	}
	batch, err := client.ExecuteBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{ChallengeID: 2, TeamID: 3, Phases: []string{"put"}})
	if err != nil || len(batch.Phases) != 1 {
		t.Fatalf("batch %+v %v", batch, err)
	}
	if requests[0].path != "/internal/v1/checkers/execute" || requests[1].path != "/internal/v1/checkers/execute-batch" {
		t.Fatalf("unexpected paths %+v", requests)
	}
	for _, request := range requests {
		if request.auth != "Bearer runner-token" || request.ctype != "application/json" {
			t.Fatalf("missing auth/content type %+v", request)
		}
	}
	var sent apigateway.CheckerExecutionRequest
	if err := json.Unmarshal([]byte(requests[0].body), &sent); err != nil || sent.Flag != "F" || sent.Phase != "check" {
		t.Fatalf("unexpected execute body %q", requests[0].body)
	}

	noToken := &httpCheckerClient{baseURL: server.URL, client: http.DefaultClient}
	if _, err := noToken.Execute(context.Background(), apigateway.CheckerExecutionRequest{}); err != nil {
		t.Fatal(err)
	}
	if requests[len(requests)-1].auth != "" {
		t.Fatal("empty token must not send an Authorization header")
	}
	if _, err := noToken.ExecuteBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPCheckerClientErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"problem detail", http.StatusInternalServerError, `{"title":"x","detail":" docker unavailable "}`, "docker unavailable"},
		{"problem title", http.StatusBadGateway, `{"title":" Runner down "}`, "Runner down"},
		{"message", http.StatusBadGateway, `{"message":"legacy"}`, "legacy"},
		{"empty", http.StatusBadGateway, `{}`, "status 502"},
		{"not json", http.StatusBadGateway, `oops`, "status 502"},
		{"bad success body", http.StatusOK, `{bad`, "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := newCheckerClient(server.URL, "")
			if _, err := client.Execute(context.Background(), apigateway.CheckerExecutionRequest{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("execute: want %q, got %v", tc.want, err)
			}
			if _, err := client.ExecuteBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("batch: want %q, got %v", tc.want, err)
			}
		})
	}
	down := &httpCheckerClient{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}
	if _, err := down.Execute(context.Background(), apigateway.CheckerExecutionRequest{}); err == nil {
		t.Fatal("transport failure must error")
	}
	if _, err := down.ExecuteBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{}); err == nil {
		t.Fatal("transport failure must error")
	}
	bad := &httpCheckerClient{baseURL: "://bad", client: http.DefaultClient}
	if _, err := bad.Execute(context.Background(), apigateway.CheckerExecutionRequest{}); err == nil {
		t.Fatal("invalid URL must error")
	}
	if _, err := bad.ExecuteBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{}); err == nil {
		t.Fatal("invalid URL must error")
	}
}

func TestExecuteBatchViaSingleChainsAndHalts(t *testing.T) {
	var metadataSeen []string
	exec := func(_ context.Context, request apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
		metadataSeen = append(metadataSeen, request.Metadata)
		switch request.Phase {
		case "put":
			return apigateway.CheckerExecutionResult{Status: "success", Output: "token-from-put"}, nil
		case "get":
			return apigateway.CheckerExecutionResult{Status: "failed", Output: "lost"}, nil
		}
		t.Fatalf("phase %q must not run after a failure", request.Phase)
		return apigateway.CheckerExecutionResult{}, nil
	}
	result, err := executeBatchViaSingle(context.Background(), exec, apigateway.CheckerBatchExecutionRequest{Metadata: "seed", Phases: []string{"put", "get", "check"}})
	if err != nil || len(result.Phases) != 2 || result.Phases[1].Status != "failed" {
		t.Fatalf("unexpected batch %+v %v", result, err)
	}
	if strings.Join(metadataSeen, ",") != "seed,token-from-put" {
		t.Fatalf("put output must chain into get metadata, got %v", metadataSeen)
	}

	boom := errors.New("runner exploded")
	_, err = executeBatchViaSingle(context.Background(), func(context.Context, apigateway.CheckerExecutionRequest) (apigateway.CheckerExecutionResult, error) {
		return apigateway.CheckerExecutionResult{}, boom
	}, apigateway.CheckerBatchExecutionRequest{Phases: []string{"put"}})
	if !errors.Is(err, boom) {
		t.Fatalf("exec error must propagate, got %v", err)
	}

	dry, err := noopCheckerClient{}.ExecuteBatch(context.Background(), apigateway.CheckerBatchExecutionRequest{Phases: []string{"put", "get"}})
	if err != nil || len(dry.Phases) != 2 {
		t.Fatalf("dry-run batch %+v %v", dry, err)
	}
}
