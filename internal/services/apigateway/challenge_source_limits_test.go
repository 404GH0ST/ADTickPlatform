package apigateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestChallengeSourceArchiveRejectsOversizedInput(t *testing.T) {
	root := t.TempDir()
	file, err := os.Create(filepath.Join(root, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxChallengeSourceArchiveBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := inspectChallengeSourceDirectory(context.Background(), root); !errors.Is(err, errChallengeSourceTooLarge) {
		t.Fatalf("oversized source should fail inspection, got %v", err)
	}

	response := httptest.NewRecorder()
	serveChallengeSourceDirectory(response, httptest.NewRequest(http.MethodGet, "/source", nil), root, "large")
	if response.Code != http.StatusRequestEntityTooLarge || response.Header().Get("Content-Type") == "application/gzip" {
		t.Fatalf("oversized archive response: status=%d headers=%v", response.Code, response.Header())
	}
}

func TestChallengeSourceArchiveRejectsTooManyEntries(t *testing.T) {
	root := t.TempDir()
	for index := 0; index <= maxChallengeSourceArchiveEntries; index++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("%05d", index)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := inspectChallengeSourceDirectory(context.Background(), root); !errors.Is(err, errChallengeSourceTooLarge) {
		t.Fatalf("excessive source entries should fail inspection, got %v", err)
	}
}

func TestChallengeSourceArchiveConcurrencyLimit(t *testing.T) {
	root := t.TempDir()
	for range maxConcurrentSourceArchives {
		challengeSourceArchiveSlots <- struct{}{}
	}
	defer func() {
		for range maxConcurrentSourceArchives {
			<-challengeSourceArchiveSlots
		}
	}()

	response := httptest.NewRecorder()
	serveChallengeSourceDirectory(response, httptest.NewRequest(http.MethodGet, "/source", nil), root, "busy")
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "5" {
		t.Fatalf("busy archive response: status=%d headers=%v", response.Code, response.Header())
	}
}
