package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"adplatform/internal/services/apigateway"
)

type checkerRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn checkerRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestHTTPCheckerValidationClientUsesExpectedRouteAndBody(t *testing.T) {
	var requestedPath string
	var requestedBody string
	client := cloneCheckerValidationClient("http://checker-runner.internal", "checker-token", &http.Client{
		Transport: checkerRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			requestedBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"challenge_id":7,"name":"proxy","checker_image":"registry.local/proxy-checker:latest","status":"valid","contract_ok":true,"checked_at":"2026-03-10T11:00:00Z","message":"checker image satisfies runner contract."}`)),
			}, nil
		}),
	})

	result, err := client.Validate(context.Background(), apigateway.CheckerValidationRequest{
		ChallengeID:  7,
		Name:         "proxy",
		CheckerImage: "registry.local/proxy-checker:latest",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestedPath != "/internal/v1/checkers/validate" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
	if !strings.Contains(requestedBody, `"challenge_id":7`) || !strings.Contains(requestedBody, `"checker_image":"registry.local/proxy-checker:latest"`) {
		t.Fatalf("unexpected request body %q", requestedBody)
	}
	if result.Status != "valid" || !result.ContractOK {
		t.Fatalf("unexpected validation result %+v", result)
	}
}
