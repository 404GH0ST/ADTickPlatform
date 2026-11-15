package main

import (
	"context"

	"adplatform/internal/services/apigateway"
)

// liveScoreboardBuild is one in-flight live scoreboard build that concurrent
// readers wait on. rows and err are written before done is closed.
type liveScoreboardBuild struct {
	generation uint64
	done       chan struct{}
	rows       []apigateway.ScoreRowAlias
	err        error
}

// liveScoreboard serves the live board with concurrent readers sharing one
// build. A build aggregates every capture, issued flag and service state, so
// without sharing, each public read and realtime poll ran its own.
//
// A reader joins an in-flight build only if no score change was committed
// through this server since it started, so a team that just had a capture
// accepted never receives a board computed before it. Shared rows are
// read-only.
func (s *gameCoreServer) liveScoreboard(ctx context.Context) ([]apigateway.ScoreRowAlias, error) {
	s.liveScoreboardMu.Lock()
	build := s.liveScoreboardBuild
	if build == nil || build.generation != s.liveScoreboardGeneration {
		build = &liveScoreboardBuild{generation: s.liveScoreboardGeneration, done: make(chan struct{})}
		s.liveScoreboardBuild = build
		go s.runLiveScoreboardBuild(build)
	}
	s.liveScoreboardMu.Unlock()

	select {
	case <-build.done:
		return build.rows, build.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// runLiveScoreboardBuild runs detached from any one reader, so a reader that
// disconnects does not fail the others waiting on the same build.
func (s *gameCoreServer) runLiveScoreboardBuild(build *liveScoreboardBuild) {
	ctx := context.Background()
	if s.scoringTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.scoringTimeout)
		defer cancel()
	}
	build.rows, build.err = s.store.ListScoreboard(ctx)

	s.liveScoreboardMu.Lock()
	if s.liveScoreboardBuild == build {
		s.liveScoreboardBuild = nil
	}
	s.liveScoreboardMu.Unlock()
	close(build.done)
}

// invalidateLiveScoreboard marks committed score changes: later readers start
// a fresh build instead of joining one that may predate the change.
func (s *gameCoreServer) invalidateLiveScoreboard() {
	s.liveScoreboardMu.Lock()
	s.liveScoreboardGeneration++
	s.liveScoreboardMu.Unlock()
}
