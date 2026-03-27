package apigateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var errWireGuardGatewayDisabled = errors.New("wireguard gateway is not configured")

type wireGuardClient interface {
	Status(ctx context.Context) (WireGuardGatewayStatus, error)
	Reconcile(ctx context.Context) (WireGuardGatewayStatus, error)
	Teardown(ctx context.Context) error
}

type noopWireGuardClient struct{}

func (noopWireGuardClient) Status(context.Context) (WireGuardGatewayStatus, error) {
	return WireGuardGatewayStatus{State: "disabled", Mode: "disabled"}, nil
}

func (noopWireGuardClient) Reconcile(context.Context) (WireGuardGatewayStatus, error) {
	return WireGuardGatewayStatus{}, errWireGuardGatewayDisabled
}

func (noopWireGuardClient) Teardown(context.Context) error {
	return nil
}

type httpWireGuardClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPWireGuardClient(baseURL, token string) wireGuardClient {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return noopWireGuardClient{}
	}
	return &httpWireGuardClient{
		baseURL: baseURL,
		token:   strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *httpWireGuardClient) Status(ctx context.Context) (WireGuardGatewayStatus, error) {
	return c.request(ctx, http.MethodGet, "/internal/v1/wireguard/status")
}

func (c *httpWireGuardClient) Reconcile(ctx context.Context) (WireGuardGatewayStatus, error) {
	return c.request(ctx, http.MethodPost, "/internal/v1/wireguard/reconcile")
}

func (c *httpWireGuardClient) Teardown(ctx context.Context) error {
	_, err := c.request(ctx, http.MethodPost, "/internal/v1/wireguard/teardown")
	return err
}

func (c *httpWireGuardClient) request(ctx context.Context, method, path string) (WireGuardGatewayStatus, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return WireGuardGatewayStatus{}, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return WireGuardGatewayStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var payload successEnvelope[WireGuardGatewayStatus]
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return WireGuardGatewayStatus{}, err
		}
		return payload.Data, nil
	}

	var payload struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return WireGuardGatewayStatus{}, fmt.Errorf("wireguard gateway request failed with status %d", resp.StatusCode)
	}
	if payload.Message != "" {
		return WireGuardGatewayStatus{}, fmt.Errorf("wireguard gateway request failed: %s", payload.Message)
	}
	return WireGuardGatewayStatus{}, fmt.Errorf("wireguard gateway request failed with status %d", resp.StatusCode)
}
