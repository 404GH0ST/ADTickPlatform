// Package servicetest helps tests drive a service's main function.
package servicetest

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// stopTimeout bounds how long RunMainUntilSIGTERM waits for main to return.
var stopTimeout = 15 * time.Second

// RunMainUntilSIGTERM runs mainFn and sends this process SIGTERM until it
// returns. A guard handler stays installed throughout, so a signal that lands
// before mainFn registers its own handler cannot kill the test binary.
func RunMainUntilSIGTERM(t testing.TB, mainFn func()) {
	t.Helper()
	guard := make(chan os.Signal, 16)
	signal.Notify(guard, syscall.SIGTERM)
	defer signal.Stop(guard)

	done := make(chan struct{})
	go func() {
		defer close(done)
		mainFn()
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(stopTimeout)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		case <-deadline:
			t.Fatal("main did not return after SIGTERM")
			return
		}
	}
}

// CaptureFatal swaps *fatal for a recorder for the duration of the test and
// returns a function reporting what it was called with.
func CaptureFatal(t testing.TB, fatal *func(...any)) func() []any {
	t.Helper()
	original := *fatal
	var got []any
	*fatal = func(args ...any) { got = append(got, args...) }
	t.Cleanup(func() { *fatal = original })
	return func() []any { return got }
}
