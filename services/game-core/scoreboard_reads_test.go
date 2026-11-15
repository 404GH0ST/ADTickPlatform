package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"adplatform/internal/services/apigateway"
)

// blockingScoreboardStore counts live builds and holds each until released.
type blockingScoreboardStore struct {
	gameStore
	builds  atomic.Int32
	release chan struct{}
}

func (s *blockingScoreboardStore) ListScoreboard(context.Context) ([]apigateway.ScoreRowAlias, error) {
	n := s.builds.Add(1)
	<-s.release
	return []apigateway.ScoreRowAlias{{Team: "Team Alpha", Rank: int(n)}}, nil
}

func TestLiveScoreboardReadersShareOneBuild(t *testing.T) {
	store := &blockingScoreboardStore{gameStore: newMemoryGameStore(), release: make(chan struct{})}
	server := newGameCoreServer("test", store, testCheckerClient{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15)

	var wg sync.WaitGroup
	results := make([][]apigateway.ScoreRowAlias, 8)
	arrived := make(chan struct{}, len(results))
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			arrived <- struct{}{}
			rows, err := server.liveScoreboard(context.Background())
			if err != nil {
				t.Error(err)
			}
			results[i] = rows
		}(i)
	}
	for range results {
		<-arrived
	}
	waitForCondition(t, time.Second, func() bool { return store.builds.Load() == 1 })
	// Every reader is past its arrival signal; give them time to join the
	// in-flight build before it is released.
	time.Sleep(100 * time.Millisecond)
	// A reader that gives up must not fail the others sharing the build.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := server.liveScoreboard(cancelled); err == nil {
		t.Fatal("expected a cancelled reader to return its context error")
	}
	close(store.release)
	wg.Wait()
	if got := store.builds.Load(); got != 1 {
		t.Fatalf("8 concurrent readers ran %d builds, want 1", got)
	}
	for i, rows := range results {
		if len(rows) != 1 || rows[0].Rank != 1 {
			t.Fatalf("reader %d got %+v", i, rows)
		}
	}
}

func TestLiveScoreboardDoesNotServeBuildsOlderThanACommittedChange(t *testing.T) {
	store := &blockingScoreboardStore{gameStore: newMemoryGameStore(), release: make(chan struct{})}
	server := newGameCoreServer("test", store, testCheckerClient{}, newFlagCodec("s", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15)

	first := make(chan []apigateway.ScoreRowAlias, 1)
	go func() {
		rows, _ := server.liveScoreboard(context.Background())
		first <- rows
	}()
	waitForCondition(t, time.Second, func() bool { return store.builds.Load() == 1 })
	server.invalidateLiveScoreboard() // e.g. a capture committed mid-build
	second := make(chan []apigateway.ScoreRowAlias, 1)
	go func() {
		rows, _ := server.liveScoreboard(context.Background())
		second <- rows
	}()
	waitForCondition(t, time.Second, func() bool { return store.builds.Load() == 2 })
	close(store.release)
	if rows := <-second; len(rows) != 1 || rows[0].Rank != 2 {
		t.Fatalf("reader after the change joined the stale build: %+v", rows)
	}
	<-first
}
