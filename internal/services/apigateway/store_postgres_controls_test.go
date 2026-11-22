package apigateway

import (
	"context"
	"reflect"
	"testing"
	"time"

	"adplatform/internal/platform/database"
	"adplatform/internal/platform/database/dbtest"
)

func TestPostgresScoreboardFreezePersistsEachWindow(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "scoreboard_freeze_test")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{}); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(db)
	now := time.Date(2026, time.November, 19, 9, 0, 0, 0, time.UTC)
	if window, err := store.GetScoreboardFreeze(ctx); err != nil || window.FreezeAt != nil {
		t.Fatalf("initial freeze window: %+v, %v", window, err)
	}
	if _, err := store.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Test Team", ContactEmail: "team@example.test"}); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListScoreboard(ctx)
	if err != nil || len(rows) != 1 || rows[0].Team != "Test Team" || rows[0].Total != 0 {
		t.Fatalf("initial scoreboard: %+v, %v", rows, err)
	}
	freezeAt := now.Add(time.Hour)
	unfreezeAt := now.Add(2 * time.Hour)
	window, err := store.SetScoreboardFreezeWindow(ctx, &freezeAt, &unfreezeAt, now)
	if err != nil || window.FreezeAt == nil || !window.FreezeAt.Equal(freezeAt) || window.UnfreezeAt == nil || !window.UnfreezeAt.Equal(unfreezeAt) {
		t.Fatalf("scheduled freeze window: %+v, %v", window, err)
	}
	first, err := store.SaveFrozenScoreboardSnapshot(ctx, rows, now)
	if err != nil || !reflect.DeepEqual(first.Snapshot, rows) || first.SnapshotTakenAt == nil || !first.SnapshotTakenAt.Equal(now) {
		t.Fatalf("first freeze snapshot: %+v, %v", first, err)
	}
	updated := []scoreRow{{Team: "Test Team", Rank: 1, Total: 42}}
	saved, err := store.SaveFrozenScoreboardSnapshot(ctx, updated, now.Add(time.Minute))
	if err != nil || !reflect.DeepEqual(saved.Snapshot, updated) || saved.SnapshotTakenAt == nil || !saved.SnapshotTakenAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("newer pre-freeze snapshot should replace the baseline: %+v, %v", saved, err)
	}
	saved, err = store.SaveFrozenScoreboardSnapshot(ctx, rows, freezeAt.Add(time.Second))
	if err != nil || !reflect.DeepEqual(saved.Snapshot, updated) {
		t.Fatalf("post-freeze snapshot must not replace the baseline: %+v, %v", saved, err)
	}
	nextFreeze := now.Add(2 * time.Hour)
	if window, err := store.SetScoreboardFreezeWindow(ctx, &nextFreeze, nil, now); err != nil || window.UnfreezeAt != nil || window.SnapshotTakenAt != nil || window.Snapshot != nil {
		t.Fatalf("replacement freeze window: %+v, %v", window, err)
	}
	if window, err := store.SaveFrozenScoreboardSnapshot(ctx, updated, nextFreeze); err != nil || !reflect.DeepEqual(window.Snapshot, updated) {
		t.Fatalf("replacement snapshot: %+v, %v", window, err)
	}
	if window, err := store.ClearScoreboardFreeze(ctx, nextFreeze); err != nil || window.FreezeAt != nil || window.UnfreezeAt != nil || window.SnapshotTakenAt != nil || window.Snapshot != nil {
		t.Fatalf("cleared freeze state: %+v, %v", window, err)
	}
}

func TestPostgresAdminAuditFilteringAndPagination(t *testing.T) {
	db := dbtest.IsolatedSchema(t, "TEST_POSTGRES_DSN", "admin_audit_test")
	ctx := context.Background()
	if err := database.RunEmbeddedMigrationsWithOptions(ctx, db, database.MigrationOptions{}); err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(db)
	for _, entry := range []adminAuditLogEntry{
		{ActorType: "organizer", Actor: "operator", Action: "challenge.create", TargetType: "challenge", Target: "draft", Status: "success"},
		{ActorType: "organizer", Actor: "operator", Action: "challenge.update", TargetType: "challenge", Target: "draft", Status: "success", Metadata: `{"field":"name"}`},
		{ActorType: "organizer", Actor: "operator", Action: "challenge.validate", TargetType: "challenge", Target: "draft", Status: "failed"},
		{ActorType: "system", Actor: "scheduler", Action: "challenge.deploy", TargetType: "challenge", Target: "draft", Status: "success"},
		{ActorType: "organizer", Actor: "operator", Action: "team.create", TargetType: "team", Target: "first team", Status: "success"},
	} {
		if err := store.AppendAdminAuditLog(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	query := adminAuditLogQuery{ActorType: " organizer ", Action: "CHALLENGE.", TargetType: "challenge", Status: "success", Limit: 1, Offset: -1}
	first, err := store.ListAdminAuditLogs(ctx, query)
	if err != nil || first.TotalCount != 2 || first.Offset != 0 || first.HasPrev || !first.HasNext || len(first.Items) != 1 || first.Items[0].Action != "challenge.update" {
		t.Fatalf("first filtered audit page: %+v, %v", first, err)
	}
	query.Offset = 1
	second, err := store.ListAdminAuditLogs(ctx, query)
	if err != nil || second.TotalCount != 2 || !second.HasPrev || second.HasNext || len(second.Items) != 1 || second.Items[0].Action != "challenge.create" || second.Items[0].Metadata != `{}` {
		t.Fatalf("second filtered audit page: %+v, %v", second, err)
	}
	all, err := store.ListAdminAuditLogs(ctx, adminAuditLogQuery{})
	if err != nil || all.TotalCount != 5 || all.Limit != 25 || len(all.Items) != 5 || all.Items[0].Action != "team.create" {
		t.Fatalf("default audit page: %+v, %v", all, err)
	}
}
