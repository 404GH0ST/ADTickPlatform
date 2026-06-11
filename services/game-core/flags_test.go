package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"adplatform/internal/platform/httpapi"
	"adplatform/internal/services/apigateway"
)

func TestFlagCodecIssuesAndParsesWithConfiguredPrefix(t *testing.T) {
	codec := newFlagCodec("test-flag-secret", "PLAYIT")
	flag := codec.Issue(101, 1, 1, 1)
	if !strings.HasPrefix(flag, "PLAYIT{") || !strings.HasSuffix(flag, "}") {
		t.Fatalf("expected PLAYIT{} wrapped flag, got %q", flag)
	}
	claims, ok := codec.Parse(flag)
	if !ok {
		t.Fatalf("expected codec to parse freshly issued flag %q", flag)
	}
	if claims.OwnerTeamID != 101 || claims.ChallengeID != 1 || claims.IssuedTick != 1 || claims.ExpiresTick != 1 {
		t.Fatalf("unexpected codec claims: %+v", claims)
	}
}

func TestFlagCodecRejectsWrongPrefix(t *testing.T) {
	codec := newFlagCodec("test-flag-secret", "PLAYIT")
	other := newFlagCodec("test-flag-secret", "FLAGv1")
	flag := other.Issue(101, 1, 1, 1)
	if _, ok := codec.Parse(flag); ok {
		t.Fatalf("expected codec to reject flag from a different prefix, got accepted %q", flag)
	}
}

func TestFlagCodecRejectsTamperedPayload(t *testing.T) {
	codec := newFlagCodec("test-flag-secret", "PLAYIT")
	flag := codec.Issue(101, 1, 1, 1)
	body := flag[len("PLAYIT{") : len(flag)-1]
	parts := strings.Split(body, ".")
	if len(parts) != 2 {
		t.Fatalf("unexpected flag body structure: %q", body)
	}
	tampered := "PLAYIT{" + parts[0] + "." + strings.Repeat("0", len(parts[1])) + "}"
	if _, ok := codec.Parse(tampered); ok {
		t.Fatalf("expected codec to reject tampered signature, got accepted %q", tampered)
	}
}

func TestFlagCodecWithPrefixFallsBackToDefault(t *testing.T) {
	codec := newFlagCodec("test-flag-secret", "   ")
	flag := codec.Issue(101, 1, 1, 1)
	if !strings.HasPrefix(flag, "PLAYIT{") {
		t.Fatalf("expected empty prefix to fall back to PLAYIT, got %q", flag)
	}
}

func TestFlagCodecReloadSwitchesActivePrefix(t *testing.T) {
	store := newMemoryGameStore()
	server := newGameCoreServer(
		"dev-admin-token",
		store,
		testCheckerClient{},
		newFlagCodec("test-flag-secret", "PLAYIT"),
		&testGameScheduler{},
		[]string{"put"},
		15,
	)

	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "game-core", Version: "dev", Addr: ":0"})
	server.RegisterRoutes(mux)

	getRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/game/flag/format", nil)
		req.Header.Set("Authorization", "Bearer dev-admin-token")
		return req
	}

	initialResponse := httptest.NewRecorder()
	mux.ServeHTTP(initialResponse, getRequest())
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("expected initial format 200, got %d", initialResponse.Code)
	}
	initial := decodeResponse[apigateway.FlagFormatStatus](t, initialResponse.Body.Bytes())
	if initial.Format != "PLAYIT" {
		t.Fatalf("expected initial format PLAYIT, got %q", initial.Format)
	}

	oldFlag := server.issueFlag(101, 1, 1, 1)
	if !strings.HasPrefix(oldFlag, "PLAYIT{") {
		t.Fatalf("expected old flag to use PLAYIT prefix, got %q", oldFlag)
	}

	refreshRequest := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/game/flag/format/refresh",
		strings.NewReader(`{"prefix":"PLAYITX"}`),
	)
	refreshRequest.Header.Set("Authorization", "Bearer dev-admin-token")
	refreshRequest.Header.Set("Content-Type", "application/json")
	refreshResponse := httptest.NewRecorder()
	mux.ServeHTTP(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("expected refresh 200, got %d: %s", refreshResponse.Code, refreshResponse.Body.String())
	}
	refreshed := decodeResponse[apigateway.FlagFormatStatus](t, refreshResponse.Body.Bytes())
	if refreshed.Format != "PLAYITX" {
		t.Fatalf("expected refreshed format PLAYITX, got %q", refreshed.Format)
	}

	if server.currentFlagFormat() != "PLAYITX" {
		t.Fatalf("expected server to report new format PLAYITX, got %q", server.currentFlagFormat())
	}

	newFlag := server.issueFlag(101, 1, 1, 1)
	if !strings.HasPrefix(newFlag, "PLAYITX{") {
		t.Fatalf("expected new flag to use PLAYITX prefix, got %q", newFlag)
	}

	if _, ok := server.parseFlag(oldFlag); ok {
		t.Fatalf("expected old PLAYIT{} flag to be rejected after reload to PLAYITX, got accepted %q", oldFlag)
	}
	if _, ok := server.parseFlag(newFlag); !ok {
		t.Fatalf("expected new PLAYITX{} flag to be accepted after reload, got rejected %q", newFlag)
	}

	badRefresh := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/game/flag/format/refresh",
		strings.NewReader(`{"prefix":""}`),
	)
	badRefresh.Header.Set("Authorization", "Bearer dev-admin-token")
	badRefresh.Header.Set("Content-Type", "application/json")
	badResponse := httptest.NewRecorder()
	mux.ServeHTTP(badResponse, badRefresh)
	if badResponse.Code != http.StatusBadRequest {
		t.Fatalf("expected empty prefix refresh 400, got %d", badResponse.Code)
	}

	authRequest := httptest.NewRequest(http.MethodPost, "/internal/v1/game/flag/format/refresh", strings.NewReader(`{"prefix":"PLAYIT"}`))
	authRequest.Header.Set("Authorization", "Bearer wrong-token")
	authResponse := httptest.NewRecorder()
	mux.ServeHTTP(authResponse, authRequest)
	if authResponse.Code != http.StatusForbidden {
		t.Fatalf("expected unauthenticated refresh 403, got %d", authResponse.Code)
	}

	_ = context.Background()
}
