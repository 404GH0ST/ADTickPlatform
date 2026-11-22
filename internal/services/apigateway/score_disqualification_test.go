package apigateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adplatform/internal/platform/httpapi"
)

func scoreboardTeams(t *testing.T, mux http.Handler) []scoreRow {
	t.Helper()
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v2/scoreboard", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("scoreboard status = %d, body %s", res.Code, res.Body.String())
	}
	var rows []scoreRow
	if err := json.NewDecoder(res.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestRankScoreRowsPlacesDisqualifiedLast(t *testing.T) {
	rows := []scoreRow{
		{Team: "Leader", Total: 500, Disqualified: true},
		{Team: "Bravo", Total: 10},
		{Team: "Alpha", Total: 10},
		{Team: "Negative", Total: -5},
		{Team: "Trailing", Total: 100, Disqualified: true},
	}
	rankScoreRows(rows)
	want := []string{"Alpha", "Bravo", "Negative", "Leader", "Trailing"}
	for index, name := range want {
		if rows[index].Team != name || rows[index].Rank != index+1 {
			t.Fatalf("position %d = %+v, want %s", index, rows[index], name)
		}
	}
}

func TestDisqualificationAppliesToFrozenBoardAndReverses(t *testing.T) {
	store, ok := NewMemoryStore(101).(*memoryStore)
	if !ok {
		t.Fatal("expected concrete memory store")
	}
	teams, err := store.ListAdminTeams(context.Background())
	if err != nil || len(teams) < 2 {
		t.Fatalf("teams: %+v %v", teams, err)
	}
	top, other := teams[0], teams[1]
	store.scoreboard = []scoreRow{
		{Rank: 1, Team: top.Name, Total: 100},
		{Rank: 2, Team: other.Name, Total: 50},
	}

	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{}, noopWireGuardClient{}).RegisterRoutes(mux)

	ctx := context.Background()
	past := time.Now().Add(-time.Minute).UTC()
	if _, err := store.SetScoreboardFreezeWindow(ctx, &past, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveFrozenScoreboardSnapshot(ctx, store.scoreboard, past.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if rows := scoreboardTeams(t, mux); rows[0].Team != top.Name {
		t.Fatalf("snapshot not captured in score order: %+v", rows)
	}
	if _, err := store.UpdateAdminTeam(ctx, top.ID, adminUpdateTeamRequest{Name: "Renamed while frozen", ContactEmail: top.ContactEmail}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.SetTeamActive(ctx, top.ID, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	rows := scoreboardTeams(t, mux)
	if rows[0].Team != other.Name || rows[1].Team != top.Name || !rows[1].Disqualified || rows[1].Rank != 2 {
		t.Fatalf("frozen board must show the DQ team last: %+v", rows)
	}
	if rows[1].Total != 100 {
		t.Fatalf("DQ must keep the frozen score: %+v", rows[1])
	}

	if _, err := store.SetTeamActive(ctx, top.ID, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	rows = scoreboardTeams(t, mux)
	if rows[0].Team != top.Name || rows[0].Disqualified {
		t.Fatalf("reinstated team must return to its rank: %+v", rows)
	}
}

func TestPublicScoreboardOmitsStableIdentityButFreezeRetainsIt(t *testing.T) {
	store := NewMemoryStore(101).(*memoryStore)
	teams, err := store.ListAdminTeams(context.Background())
	if err != nil || len(teams) == 0 {
		t.Fatalf("teams: %v", err)
	}
	team := teams[0]
	store.scoreboard = []scoreRow{{Rank: 1, Team: team.Name, TeamIdentity: team.ScoreIdentity, Total: 100}}
	mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "dev", Addr: ":0"})
	NewWithDeps("dev-team-token", "dev-admin-token", 101, store, testControllerClient{}, noopWireGuardClient{}).RegisterRoutes(mux)
	assertPublic := func() {
		t.Helper()
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v2/scoreboard", nil))
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "team_identity") || strings.Contains(response.Body.String(), team.ScoreIdentity) {
			t.Fatalf("public scoreboard exposed stable identity: %d %s", response.Code, response.Body.String())
		}
	}
	assertPublic()
	past := time.Now().Add(-time.Minute).UTC()
	if _, err := store.SetScoreboardFreezeWindow(context.Background(), &past, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveFrozenScoreboardSnapshot(context.Background(), store.scoreboard, past.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	assertPublic()
	freeze, err := store.GetScoreboardFreeze(context.Background())
	if err != nil || len(freeze.Snapshot) != 1 || freeze.Snapshot[0].TeamIdentity != team.ScoreIdentity {
		t.Fatalf("freeze lost internal identity: %+v %v", freeze, err)
	}
	assertPublic()
}

func TestDisqualifiedTeamIsNotRedeployed(t *testing.T) {
	store, ok := NewMemoryStore(101).(*memoryStore)
	if !ok {
		t.Fatal("expected concrete memory store")
	}
	ctx := context.Background()
	teams, err := store.ListAdminTeams(ctx)
	if err != nil || len(teams) == 0 {
		t.Fatalf("teams: %+v %v", teams, err)
	}
	target := teams[0].ID
	if _, err := store.SetTeamActive(ctx, target, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.RequeueTeamServices(ctx, target); err != nil {
		t.Fatal(err)
	}
	tasks, err := store.ListControllerRuntimeTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.TeamID == target {
			t.Fatalf("disqualified team received a runtime task: %+v", task)
		}
	}
}
