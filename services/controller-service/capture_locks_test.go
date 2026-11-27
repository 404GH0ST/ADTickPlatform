package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCaptureLocksDoNotCrossInstances(t *testing.T) {
	e := &dockerCLIExecutor{}
	unlock := e.lockCaptureInstance("team-a")
	defer unlock()
	acquired := make(chan struct{})
	go func() {
		defer close(acquired)
		e.lockCaptureInstance("team-b")()
	}()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("a hung operation on one team must not block another team")
	}
	if e.captureInstanceLock("team-a").TryLock() {
		t.Fatal("one instance must still run one operation at a time")
	}
}

func TestCaptureSweepSkipsBusyInstance(t *testing.T) {
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	binary := filepath.Join(dir, "docker")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + calls + "'\n[ \"$1\" = ps ] && echo busy-instance\nexit 0\n"
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	e := &dockerCLIExecutor{binary: binary, captureMode: "guarded", captureVolume: "capture", timeout: time.Second}
	defer e.lockCaptureInstance("busy-instance")()
	if err := e.sweepCapture(context.Background()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	// A busy instance may be inspected as part of the batch, but must never be
	// mutated (stopped, restarted, or exec'd into) while another operation holds
	// its lock.
	if strings.Contains(string(body), "stop") || strings.Contains(string(body), "restart") || strings.Contains(string(body), "exec") {
		t.Fatalf("the sweep must not mutate a busy instance: %s", body)
	}
}
