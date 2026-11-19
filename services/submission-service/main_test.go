package main

import (
	"context"
	"testing"
	"time"

	"adplatform/internal/platform/servicetest"
)

func TestRunServesUntilCancelled(t *testing.T) {
	t.Setenv("SUBMISSION_SERVICE_ADDR", "127.0.0.1:0")
	t.Setenv("ADMIN_API_TOKEN", "unit-test-admin-token-7f3a")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := run(ctx); err != nil {
		t.Fatalf("run must shut down cleanly, got %v", err)
	}
}

func TestMainStopsOnSIGTERM(t *testing.T) {
	t.Setenv("SUBMISSION_SERVICE_ADDR", "127.0.0.1:0")
	t.Setenv("ADMIN_API_TOKEN", "unit-test-admin-token-7f3a")
	got := servicetest.CaptureFatal(t, &fatal)
	servicetest.RunMainUntilSIGTERM(t, main)
	if len(got()) != 0 {
		t.Fatalf("clean shutdown must not be fatal, got %v", got())
	}
}

func TestMainReportsListenFailure(t *testing.T) {
	t.Setenv("SUBMISSION_SERVICE_ADDR", "no-port-here")
	t.Setenv("ADMIN_API_TOKEN", "unit-test-admin-token-7f3a")
	got := servicetest.CaptureFatal(t, &fatal)
	main()
	if len(got()) == 0 {
		t.Fatal("a listen failure must be fatal")
	}
}
