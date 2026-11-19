package servicetest

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestRunMainUntilSIGTERMStopsSignalAwareMain(t *testing.T) {
	ran := false
	RunMainUntilSIGTERM(t, func() {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
		defer stop()
		<-ctx.Done()
		ran = true
	})
	if !ran {
		t.Fatal("main body did not complete")
	}
}

func TestCaptureFatalRecordsAndRestores(t *testing.T) {
	var fatal = func(...any) { os.Exit(99) }
	original := &fatal
	t.Run("inner", func(t *testing.T) {
		got := CaptureFatal(t, original)
		(*original)("boom", 1)
		if args := got(); len(args) != 2 || args[0] != "boom" {
			t.Fatalf("recorded %v", args)
		}
	})
}

type fatalRecorder struct {
	testing.TB
	fatals int
}

func (f *fatalRecorder) Helper() {}
func (f *fatalRecorder) Fatal(...any) {
	f.fatals++
}

func TestRunMainUntilSIGTERMReportsAHungMain(t *testing.T) {
	original := stopTimeout
	stopTimeout = 150 * time.Millisecond
	t.Cleanup(func() { stopTimeout = original })
	release := make(chan struct{})
	defer close(release)
	rec := &fatalRecorder{TB: t}
	RunMainUntilSIGTERM(rec, func() { <-release }) // ignores SIGTERM
	if rec.fatals != 1 {
		t.Fatalf("a main that never returns must be reported, got %d", rec.fatals)
	}
}
