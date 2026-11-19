package database_test

import (
	"context"
	"testing"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

// On a fresh schema the first round trips are: advisory lock, ensure
// schema_migrations, then for the first migration lookup, begin, apply,
// record, and commit. Each failure must abort, and a clean retry must finish.
func TestMigrationsRecoverFromFaultAtEachStep(t *testing.T) {
	for position := 1; position <= 7; position++ {
		db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "migrate_faults")
		faulty, faults := dbtest.FaultInjectingDB(t, "TEST_POSTGRES_DSN", db)
		faults.Arm(position)
		if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), faulty, database.MigrationOptions{}); err == nil {
			t.Fatalf("fault at round trip %d must abort migrations", position)
		}
		faults.Arm(0)
		if err := database.RunEmbeddedMigrationsWithOptions(context.Background(), faulty, database.MigrationOptions{}); err != nil {
			t.Fatalf("retry after fault at round trip %d: %v", position, err)
		}
	}
}
