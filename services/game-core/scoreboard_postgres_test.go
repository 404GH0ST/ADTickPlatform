package main

import (
	"context"
	"testing"

	"adplatform/internal/platform/database"
)

// A deactivation that commits while a recompute is between reading active
// teams and upserting can re-insert the team's row. The next recompute must
// remove it; previously only deleted teams were swept.
func TestPostgresRecomputeDropsDeactivatedTeamRows(t *testing.T) {
	db := isolatedSubmissionDB(t)
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{IncludeSeeds: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO teams(id,name,email,join_key,active) VALUES(101,'Alpha','a@t','a',TRUE),(102,'Bravo','b@t','b',TRUE),(103,'Charlie','c@t','c',FALSE);
		INSERT INTO scoreboard_entries(team_id,rank,team_name,attack_points,defense_points,sla_points,total_points,delta)
		VALUES(103,1,'Charlie',5,0,0,5,'same');
	`); err != nil {
		t.Fatal(err)
	}
	store := &postgresGameStore{db: db}
	if _, err := store.RecomputeScoreboard(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := store.listStoredScoreboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range stored {
		if row.Team == "Charlie" {
			t.Fatalf("deactivated team still stored after recompute: %+v", stored)
		}
	}
	audit, err := store.AuditScoreboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if audit.MismatchCount != 0 {
		t.Fatalf("scoring audit reports mismatches: %+v", audit)
	}
}
