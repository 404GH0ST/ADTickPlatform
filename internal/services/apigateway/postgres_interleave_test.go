package apigateway

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

// domainErrors are the outcomes a store operation may report when a
// concurrent change wins the race; anything else is a defect.
var domainErrors = []error{ErrChallengeNotFound, ErrServiceUnavailable, ErrChallengeMaintenance, ErrServiceLocked, ErrTeamNotFound, ErrPlayerNotFound, ErrInvalidCredentials, ErrDuplicateResource}

func isDomainError(err error) bool {
	for _, domain := range domainErrors {
		if errors.Is(err, domain) {
			return true
		}
	}
	return false
}

// sweepInterleaved runs op once per round trip with change applied just
// before that round trip, as if another session had committed it then.
func sweepInterleaved(t *testing.T, faults *dbtest.Faults, name string, op func() error, change, restore func()) {
	t.Helper()
	restore()
	faults.Arm(0)
	if err := op(); err != nil {
		t.Fatalf("%s: clean run: %v", name, err)
	}
	total := faults.Count()
	sawDomainError := false
	for position := 1; position <= total; position++ {
		restore()
		faults.ArmHook(position, change)
		err := op()
		faults.Arm(0)
		if err != nil {
			if !isDomainError(err) {
				t.Errorf("%s: change before round trip %d/%d gave a non-domain error: %v", name, position, total, err)
			}
			sawDomainError = true
		}
	}
	restore()
	if !sawDomainError {
		t.Errorf("%s: no interleaving was ever noticed", name)
	}
}

func TestPostgresStoreSurvivesInterleavedChanges(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "gateway_interleave_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "interleave-server-public-key")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	faulty, faults := dbtest.FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	clean, store := NewPostgresStore(db), NewPostgresStore(faulty)
	now := time.Now()
	// Hook statements may wait on rows the operation holds; a timeout lets
	// them give up instead of deadlocking against the paused operation.
	exec := func(statement string, args ...any) {
		hookCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_, _ = db.ExecContext(hookCtx, statement, args...)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	team, err := clean.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Interleave", ContactEmail: "interleave@example.test"})
	must(err)
	challenge, err := clean.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "interleave-web", BaselineImage: "i", CheckerImage: "c"}, now)
	must(err)
	_, err = clean.DeployAdminChallenge(ctx, challenge.ID)
	must(err)
	_, err = clean.ReconcileAdminDeployments(ctx, now)
	must(err)
	_, err = db.Exec(`UPDATE challenges SET published = TRUE;
		INSERT INTO game_match_state(singleton, state) VALUES (TRUE, 'running') ON CONFLICT (singleton) DO UPDATE SET state = 'running';
		UPDATE team_service_states SET unlocked = TRUE;
		CREATE TABLE saved_instance AS SELECT * FROM service_instances`)
	must(err)
	reopen := func() {
		exec(`UPDATE challenges SET maintenance = FALSE WHERE id = $1`, challenge.ID)
		exec(`INSERT INTO service_instances SELECT * FROM saved_instance ON CONFLICT DO NOTHING`)
		exec(`UPDATE team_service_states SET status = 'stable' WHERE team_id = $1 AND challenge_id = $2`, team.ID, challenge.ID)
	}
	closeChallenge := func() { exec(`UPDATE challenges SET maintenance = TRUE WHERE id = $1`, challenge.ID) }
	dropInstance := func() {
		exec(`DELETE FROM service_instances WHERE team_id = $1 AND challenge_id = $2`, team.ID, challenge.ID)
	}

	sweepInterleaved(t, faults, "ssh session vs maintenance", func() error {
		_, err := store.CreateSSHSession(ctx, team.ID, challenge.ID, now)
		return err
	}, closeChallenge, reopen)
	for name, change := range map[string]func(){"reset vs maintenance": closeChallenge, "reset vs instance removal": dropInstance} {
		sweepInterleaved(t, faults, name, func() error {
			prepared, err := store.PrepareFactoryResetService(ctx, team.ID, challenge.ID)
			if err == nil {
				err = clean.AbortFactoryResetService(ctx, prepared)
			}
			return err
		}, change, reopen)
	}

	// Each run below works on its own throwaway rows, so the change may delete.
	var current int
	run := 0
	sweepInterleaved(t, faults, "resume vs deletion", func() error {
		run++
		created, err := clean.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: fmt.Sprintf("interleave-resume-%d", run), BaselineImage: "i", CheckerImage: "c", ServiceSubnetOctet: 100 + run}, now)
		if err != nil {
			return err
		}
		current = created.ID
		_, err = store.SetChallengeMaintenance(ctx, created.ID, false, now)
		return err
	}, func() { exec(`DELETE FROM challenges WHERE id = $1`, current) }, func() {})

	sweepInterleaved(t, faults, "player creation vs team deletion", func() error {
		run++
		fresh, err := clean.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: fmt.Sprintf("Doomed %d", run), ContactEmail: fmt.Sprintf("doomed-%d@example.test", run)})
		if err != nil {
			return err
		}
		current = fresh.ID
		_, err = store.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: fresh.ID, DisplayName: "P", Email: fmt.Sprintf("doomed-player-%d@example.test", run), Password: "interleave-password"}, now)
		return err
	}, func() { exec(`DELETE FROM teams WHERE id = $1`, current) }, func() {})

	sweepInterleaved(t, faults, "peer read vs player deletion", func() error {
		run++
		player, err := clean.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: team.ID, DisplayName: "Gone", Email: fmt.Sprintf("gone-%d@example.test", run), Password: "interleave-password"}, now)
		if err != nil {
			return err
		}
		current = player.ID
		_, err = store.GetAdminPlayerWireGuardConfig(ctx, player.ID)
		return err
	}, func() { exec(`DELETE FROM players WHERE id = $1`, current) }, func() {})

	var member adminPlayer
	sweepInterleaved(t, faults, "profile vs team deletion", func() error {
		run++
		fresh, err := clean.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: fmt.Sprintf("Interleave %d", run), ContactEmail: fmt.Sprintf("interleave-%d@example.test", run)})
		if err != nil {
			return err
		}
		if _, err := clean.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: fresh.ID, DisplayName: "Lead", Email: fmt.Sprintf("lead-%d@example.test", run), Password: "interleave-password", Role: "captain"}, now); err != nil {
			return err
		}
		member, err = clean.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: fresh.ID, DisplayName: "Mem", Email: fmt.Sprintf("mem-%d@example.test", run), Password: "interleave-password", Role: "member"}, now)
		if err != nil {
			return err
		}
		current = fresh.ID
		_, err = store.UpdateParticipantProfile(ctx, member.ID, participantUpdateProfileRequest{DisplayName: "Mem2", Email: member.Email})
		return err
	}, func() { exec(`DELETE FROM teams WHERE id = $1`, current) }, func() {})
}

func TestPostgresStoreFreshSchemaFaults(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "gateway_fresh_faults_test")
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "fresh-server-public-key")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	faulty, faults := dbtest.FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
	clean, store := NewPostgresStore(db), NewPostgresStore(faulty)
	now := time.Now()
	challenge, err := clean.CreateAdminChallenge(ctx, adminCreateChallengeRequest{Name: "fresh-web", BaselineImage: "i", CheckerImage: "c"}, now)
	if err != nil {
		t.Fatal(err)
	}
	ops := map[string]func(run int) error{
		// With no teams there is no deployment job to create.
		"DeployWithoutTeams": func(int) error {
			if _, err := db.ExecContext(ctx, `DELETE FROM teams`); err != nil {
				return err
			}
			_, err := store.DeployAdminChallenge(ctx, challenge.ID)
			return err
		},
		// With no ticks a reactivated team plays immediately.
		"ReactivateWithoutTicks": func(run int) error {
			team, err := clean.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: fmt.Sprintf("Fresh %d-%d", run, time.Now().UnixNano()), ContactEmail: fmt.Sprintf("fresh-%d-%d@example.test", run, time.Now().UnixNano())})
			if err != nil {
				return err
			}
			if _, err := clean.SetTeamActive(ctx, team.ID, false, now); err != nil {
				return err
			}
			_, err = store.SetTeamActive(ctx, team.ID, true, now)
			return err
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			dbtest.SweepFaults(t, faults, name, op)
			dbtest.SweepRowFaults(t, faults, name, op)
			dbtest.SweepResultFaults(t, faults, name, op)
		})
	}
}

func TestPostgresLifecycleLockSlots(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "gateway_lifecycle_slots_test")
	db.SetMaxOpenConns(2)
	store := NewPostgresStore(db).(*postgresStore)
	unlock, err := store.LockServiceLifecycle(context.Background(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if cap(store.lifecycleSlots) != 1 {
		t.Fatalf("a two-connection pool leaves one lifecycle slot, got %d", cap(store.lifecycleSlots))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.LockServiceLifecycle(ctx, 2, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting for a slot ends with the caller's context: %v", err)
	}
}
