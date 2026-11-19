package main

import (
	"context"
	"errors"
	"testing"

	"adplatform/internal/services/apigateway"
)

// completeTickFailures fails the first n CompleteTick calls, like a database
// blip right as a tick is being persisted.
type completeTickFailures struct {
	gameStore
	remaining int
}

func (s *completeTickFailures) CompleteTick(ctx context.Context, tick apigateway.GameTickStatus) (apigateway.GameTickStatus, error) {
	if s.remaining > 0 {
		s.remaining--
		return apigateway.GameTickStatus{}, errors.New("database blip")
	}
	return s.gameStore.CompleteTick(ctx, tick)
}

// listTargetsFailsOnce fails the first target listing so the tick fails.
type listTargetsFailsOnce struct {
	*completeTickFailures
	failed bool
}

func (s *listTargetsFailsOnce) ListCheckerTargets(ctx context.Context) ([]checkerTarget, error) {
	if !s.failed {
		s.failed = true
		return nil, errors.New("database blip")
	}
	return s.completeTickFailures.ListCheckerTargets(ctx)
}

func newTickServer(store gameStore) *gameCoreServer {
	return newGameCoreServer("dev-admin-token", store, testCheckerClient{}, newFlagCodec("test-flag-secret", "PLAYIT"), &testGameScheduler{}, []string{"put", "get", "check"}, 15).
		WithWarmupRequired(false).
		WithAutoTickOnMatchStart(false)
}

// A tick whose completion could not be persisted must not wedge the game:
// the next tick finalizes it and proceeds instead of reporting a tick that
// will never finish.
func TestUnfinalizedTickDoesNotWedgeTheGame(t *testing.T) {
	ctx := context.Background()
	for name, store := range map[string]gameStore{
		"completed tick not persisted": &completeTickFailures{gameStore: newMemoryGameStore(), remaining: 1},
		"failed tick not persisted":    &listTargetsFailsOnce{completeTickFailures: &completeTickFailures{gameStore: newMemoryGameStore(), remaining: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			server := newTickServer(store)
			if _, err := store.StartMatch(ctx, server.now()); err != nil {
				t.Fatal(err)
			}
			if _, err := server.advanceTick(ctx); err == nil {
				t.Fatal("the blipped tick must report its failure")
			}
			next, err := server.advanceTick(ctx)
			if err != nil {
				t.Fatalf("the game must recover on the next tick, got %v", err)
			}
			if next.ID != 2 {
				t.Fatalf("the next tick must follow the unfinalized one, got tick %d", next.ID)
			}
			status, err := store.GameStatus(ctx)
			if err != nil || status.TotalTicks != 2 {
				t.Fatalf("both ticks must be finalized: %+v %v", status, err)
			}
		})
	}
}

// While the database stays down the retry keeps failing, and no new tick
// starts on top of the unfinalized one.
func TestUnfinalizedTickRetryFailureBlocksNewTicks(t *testing.T) {
	ctx := context.Background()
	store := &completeTickFailures{gameStore: newMemoryGameStore(), remaining: 2}
	server := newTickServer(store)
	if _, err := store.StartMatch(ctx, server.now()); err != nil {
		t.Fatal(err)
	}
	_, _ = server.advanceTick(ctx)
	if _, err := server.advanceTick(ctx); err == nil {
		t.Fatal("a still-failing finalization must be reported")
	}
	if next, err := server.advanceTick(ctx); err != nil || next.ID != 2 {
		t.Fatalf("once the database recovers the game continues: %+v %v", next, err)
	}
}
