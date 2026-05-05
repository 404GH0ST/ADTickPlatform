package apigateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPControllerClientFactoryResetUsesExpectedRoute(t *testing.T) {
	var requestedPath string
	var requestedAuth string
	client := NewHTTPControllerClient("http://controller.internal", "controller-token")
	httpClient := client.(*httpControllerClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			requestedAuth = request.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"success","data":{}}`)),
			}, nil
		}),
	}

	if err := client.FactoryResetService(context.Background(), 101, 3); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if requestedPath != "/internal/v1/teams/101/services/3/reset/factory" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
	if requestedAuth != "Bearer controller-token" {
		t.Fatalf("unexpected auth header %s", requestedAuth)
	}
}

func TestHTTPControllerClientReconcileDeploymentsUsesExpectedRoute(t *testing.T) {
	var requestedPath string
	client := NewHTTPControllerClient("http://controller.internal", "controller-token")
	httpClient := client.(*httpControllerClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"processed_jobs":1,"processed_instances":4,"completed_jobs":1}`)),
			}, nil
		}),
	}

	result, err := client.ReconcileDeployments(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestedPath != "/internal/v1/deployments/reconcile" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
	if result.ProcessedJobs != 1 || result.ProcessedInstances != 4 || result.CompletedJobs != 1 {
		t.Fatalf("unexpected result %+v", result)
	}
}

func TestHTTPControllerClientMapsNotFoundToChallengeError(t *testing.T) {
	client := NewHTTPControllerClient("http://controller.internal", "controller-token")
	httpClient := client.(*httpControllerClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"failed","message":"service runtime was not found."}`)),
			}, nil
		}),
	}

	err := client.RestartService(context.Background(), 101, 3)
	if !errors.Is(err, ErrChallengeNotFound) {
		t.Fatalf("expected ErrChallengeNotFound, got %v", err)
	}
}

func TestHTTPControllerClientAccessStatusUsesExpectedRoute(t *testing.T) {
	var requestedPath string
	client := NewHTTPControllerClient("http://controller.internal", "controller-token")
	httpClient := client.(*httpControllerClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"state":"applied","mode":"dry-run","policies_total":12,"ssh_open_services":4,"ssh_locked_services":8,"allowed_peers_total":6}`)),
			}, nil
		}),
	}

	status, err := client.AccessStatus(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestedPath != "/internal/v1/access/status" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
	if status.PoliciesTotal != 12 || status.SSHOpenServices != 4 {
		t.Fatalf("unexpected status %+v", status)
	}
}

func TestHTTPControllerClientApplySSHCredentialUsesExpectedRouteAndBody(t *testing.T) {
	var requestedPath string
	var requestedBody string
	client := NewHTTPControllerClient("http://controller.internal", "controller-token")
	httpClient := client.(*httpControllerClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			requestedBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"success","data":{}}`)),
			}, nil
		}),
	}

	err := client.ApplySSHCredential(context.Background(), 101, 1, ControllerSSHCredential{
		Password: "one-time-secret",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestedPath != "/internal/v1/teams/101/services/1/ssh-credential" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
	if !strings.Contains(requestedBody, `"password":"one-time-secret"`) || strings.Contains(requestedBody, `"expires_at":`) {
		t.Fatalf("unexpected request body %q", requestedBody)
	}
}

func TestHTTPControllerClientValidateChallengeRuntimeUsesExpectedRouteAndBody(t *testing.T) {
	var requestedPath string
	var requestedBody string
	client := NewHTTPControllerClient("http://controller.internal", "controller-token")
	httpClient := client.(*httpControllerClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			body, err := io.ReadAll(request.Body)
			if err != nil {
				return nil, err
			}
			requestedBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"challenge_id":7,"name":"proxy","baseline_image":"registry.local/proxy:baseline","checker_image":"registry.local/proxy-checker:latest","status":"valid","baseline_ssh_contract_ok":true,"checker_contract_ok":true,"checked_at":"2026-03-10T09:30:00Z","message":"challenge package satisfies runtime validation."}`)),
			}, nil
		}),
	}

	result, err := client.ValidateChallengeRuntime(context.Background(), ChallengeValidationRequest{
		ChallengeID:   7,
		Name:          "proxy",
		BaselineImage: "registry.local/proxy:baseline",
		CheckerImage:  "registry.local/proxy-checker:latest",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestedPath != "/internal/v1/challenges/validate" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
	if !strings.Contains(requestedBody, `"challenge_id":7`) || !strings.Contains(requestedBody, `"baseline_image":"registry.local/proxy:baseline"`) {
		t.Fatalf("unexpected request body %q", requestedBody)
	}
	if result.Status != "valid" || !result.BaselineSSHContractOK || !result.CheckerContractOK {
		t.Fatalf("unexpected validation result %+v", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
