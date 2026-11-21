package apigateway

import (
	"context"
	"net/http"
	"testing"
)

func TestNormalizeChallengeCategoryAndProtocol(t *testing.T) {
	categories := []struct {
		raw, want string
		ok        bool
	}{
		{"", "", true},
		{" PWN ", "pwn", true},
		{"web", "web", true},
		{"Crypto", "crypto", true},
		{"misc", "", false},
		{"web exploitation", "", false},
	}
	for _, tc := range categories {
		got, ok := normalizeChallengeCategory(tc.raw)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("normalizeChallengeCategory(%q) = %q, %v; want %q, %v", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
	protocols := []struct {
		raw, want string
		ok        bool
	}{
		{"", "tcp", true},
		{"TCP", "tcp", true},
		{" http ", "http", true},
		{"https", "", false},
		{"udp", "", false},
	}
	for _, tc := range protocols {
		got, ok := normalizeChallengeProtocol(tc.raw)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("normalizeChallengeProtocol(%q) = %q, %v; want %q, %v", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
	if got := storedChallengeCategory("misc"); got != "" {
		t.Fatalf("storedChallengeCategory must drop unknown values, got %q", got)
	}
	if got := storedChallengeProtocol("udp"); got != "tcp" {
		t.Fatalf("storedChallengeProtocol must fall back to tcp, got %q", got)
	}
}

func TestAdminChallengeCategoryProtocolCreateUpdateAndPublicList(t *testing.T) {
	mux, store := newInstitutionTestServer(t)

	created := serveInstitutionRequest(t, mux, http.MethodPost, "/api/v2/admin/challenges",
		`{"name":"heap-notes","category":"PWN","protocol":"tcp"}`, true)
	if created.Code != http.StatusOK {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	challenge := decodeCompat[adminChallenge](t, created.Body.Bytes())
	if challenge.Category != "pwn" || challenge.Protocol != "tcp" {
		t.Fatalf("create = %q/%q, want pwn/tcp", challenge.Category, challenge.Protocol)
	}

	// Omitting both keeps them; sending them changes them.
	kept := serveInstitutionRequest(t, mux, http.MethodPut, "/api/v2/admin/challenges/4",
		`{"name":"heap-notes-v2"}`, true)
	if kept.Code != http.StatusOK {
		t.Fatalf("update: %d %s", kept.Code, kept.Body.String())
	}
	if got := decodeCompat[adminChallenge](t, kept.Body.Bytes()); got.Category != "pwn" || got.Protocol != "tcp" {
		t.Fatalf("update without metadata must keep it, got %q/%q", got.Category, got.Protocol)
	}
	changed := serveInstitutionRequest(t, mux, http.MethodPut, "/api/v2/admin/challenges/4",
		`{"name":"heap-notes-v2","category":"web","protocol":"http"}`, true)
	if got := decodeCompat[adminChallenge](t, changed.Body.Bytes()); got.Category != "web" || got.Protocol != "http" {
		t.Fatalf("update = %q/%q, want web/http", got.Category, got.Protocol)
	}
	cleared := serveInstitutionRequest(t, mux, http.MethodPut, "/api/v2/admin/challenges/4",
		`{"name":"heap-notes-v2","category":""}`, true)
	if got := decodeCompat[adminChallenge](t, cleared.Body.Bytes()); got.Category != "" || got.Protocol != "http" {
		t.Fatalf("an empty category must clear it and keep the protocol, got %q/%q", got.Category, got.Protocol)
	}

	// Challenges that predate the fields read back as uncategorized TCP.
	list, err := store.ListAdminChallenges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Category != "" || list[0].Protocol != "tcp" {
		t.Fatalf("legacy challenge = %q/%q, want uncategorized tcp", list[0].Category, list[0].Protocol)
	}

	// Participants see both on the public catalog once it is deployed (published).
	if _, err := store.DeployAdminChallenge(context.Background(), 4); err != nil {
		t.Fatal(err)
	}
	public, err := store.ListChallenges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range public {
		if item.ID == 4 {
			found = true
			if item.Category != "" || item.Protocol != "http" {
				t.Fatalf("public challenge = %q/%q, want uncategorized http", item.Category, item.Protocol)
			}
		}
	}
	if !found {
		t.Fatalf("published challenge missing from the public catalog: %+v", public)
	}
}

func TestAdminChallengeRejectsUnknownCategoryOrProtocol(t *testing.T) {
	mux, _ := newInstitutionTestServer(t)
	for _, body := range []string{
		`{"name":"bad-cat","category":"misc"}`,
		`{"name":"bad-proto","protocol":"udp"}`,
	} {
		if response := serveInstitutionRequest(t, mux, http.MethodPost, "/api/v2/admin/challenges", body, true); response.Code != http.StatusBadRequest {
			t.Fatalf("create %s: want 400, got %d %s", body, response.Code, response.Body.String())
		}
	}
	for _, body := range []string{
		`{"name":"banking","category":"forensics"}`,
		`{"name":"banking","protocol":"https"}`,
	} {
		if response := serveInstitutionRequest(t, mux, http.MethodPut, "/api/v2/admin/challenges/1", body, true); response.Code != http.StatusBadRequest {
			t.Fatalf("update %s: want 400, got %d %s", body, response.Code, response.Body.String())
		}
	}
}
