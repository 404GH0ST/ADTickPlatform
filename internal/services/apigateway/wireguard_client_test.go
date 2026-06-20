package apigateway

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPWireGuardClientReconcileUsesExpectedRoute(t *testing.T) {
	var requestedPath string
	var requestedAuth string
	client := NewHTTPWireGuardClient("http://wireguard.internal", "wireguard-token")
	httpClient := client.(*httpWireGuardClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			requestedAuth = request.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"state":"applied","mode":"dry-run","peers_total":4,"peers_active":4,"peers_revoked":0}`)),
			}, nil
		}),
	}

	status, err := client.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestedPath != "/internal/v1/wireguard/reconcile" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
	if requestedAuth != "Bearer wireguard-token" {
		t.Fatalf("unexpected auth header %s", requestedAuth)
	}
	if status.State != "applied" || status.PeersTotal != 4 {
		t.Fatalf("unexpected status %+v", status)
	}
}

func TestHTTPWireGuardClientTeardownAcceptsNoContent(t *testing.T) {
	var requestedPath string
	client := NewHTTPWireGuardClient("http://wireguard.internal", "wireguard-token")
	httpClient := client.(*httpWireGuardClient)
	httpClient.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestedPath = request.URL.Path
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	if err := client.Teardown(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if requestedPath != "/internal/v1/wireguard/teardown" {
		t.Fatalf("unexpected path %s", requestedPath)
	}
}
