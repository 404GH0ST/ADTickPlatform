package main

import (
	"errors"
	"os/exec"
	"sync"
)

// maxCheckerProcessOutputBytes caps combined stdout/stderr retained per Docker
// command. The writer still accepts every byte so a noisy child cannot block on
// a full pipe while the retained host memory remains bounded.
const maxCheckerProcessOutputBytes = 1 << 20

var errCheckerOutputLimit = errors.New("checker command output exceeded limit")

type boundedOutputBuffer struct {
	mu       sync.Mutex
	output   []byte
	limit    int
	exceeded bool
}

func (b *boundedOutputBuffer) Write(p []byte) (int, error) {
	written := len(p)
	b.mu.Lock()
	defer b.mu.Unlock()

	remaining := b.limit - len(b.output)
	if remaining > 0 {
		keep := len(p)
		if keep > remaining {
			keep = remaining
		}
		b.output = append(b.output, p[:keep]...)
	}
	if written > remaining {
		b.exceeded = true
	}
	return written, nil
}

func (b *boundedOutputBuffer) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.output, b.exceeded
}

func runCommandWithBoundedOutput(cmd *exec.Cmd, limit int) ([]byte, bool, error) {
	if limit <= 0 {
		return nil, false, errors.New("checker output limit must be positive")
	}
	capture := &boundedOutputBuffer{limit: limit}
	cmd.Stdout = capture
	cmd.Stderr = capture
	err := cmd.Run()
	output, exceeded := capture.snapshot()
	return output, exceeded, err
}
