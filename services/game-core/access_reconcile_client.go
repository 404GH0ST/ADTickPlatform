package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type httpAccessReconcileClient struct {
	baseURL    string
	adminToken string
	client     *http.Client
}

func newAccessReconcileClient(baseURL, adminToken string) accessReconcileClient {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return nil
	}
	return &httpAccessReconcileClient{
		baseURL:    trimmed,
		adminToken: strings.TrimSpace(adminToken),
		client:     &http.Client{Timeout: 15 * time.Second},
	}
}

// ReconcileAccess re-applies controller service access policies. In host mode the
// controller also best-effort converges the WireGuard gateway so tick-opened
// (play_from_tick) challenges become reachable without a manual admin reconcile.
func (c *httpAccessReconcileClient) ReconcileAccess(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/access/reconcile", nil)
	if err != nil {
		return err
	}
	if c.adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.adminToken)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("access reconcile returned %d", resp.StatusCode)
	}
	return nil
}
