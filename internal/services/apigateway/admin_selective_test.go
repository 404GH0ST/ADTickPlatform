package apigateway

import (
	"net/http"
	"testing"

	"adplatform/internal/platform/httpapi"
)

type adminSelectiveCase struct {
	name         string
	method, path string
	body         any
	store        failSet
	controller   failSet
	wireGuard    failSet
	game         failSet
	scoring      failSet
	matchState   string
	validation   ChallengeValidationResult
	want         int
}

// TestAdminRoutesReachLaterFailureBranches fails only the named dependency
// methods, so each handler gets past its earlier steps to the branch under
// test.
func TestAdminRoutesReachLaterFailureBranches(t *testing.T) {
	disabledScoring := failSet{"Scoreboard": errScoringWorkerDisabled, "RecomputeScoring": errScoringWorkerDisabled, "AuditScoring": errScoringWorkerDisabled}
	cases := []adminSelectiveCase{
		{name: "team delete fails and compensation fails", method: http.MethodDelete, path: "/api/v2/admin/teams/101",
			store: failSet{"DeleteAdminTeam": errStoreDown}, controller: failSet{"ReconcileDeployments": errUpstreamDown}, want: http.StatusInternalServerError},
		{name: "team deactivation access revocation fails", method: http.MethodPost, path: "/api/v2/admin/teams/101/deactivate",
			controller: failSet{"ReconcileAccessPolicies": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "team reactivation access fails", method: http.MethodPost, path: "/api/v2/admin/teams/101/reactivate",
			controller: failSet{"ReconcileAccessPolicies": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "rotate then gateway fails", method: http.MethodPost, path: "/api/v2/admin/players/1/wireguard/rotate",
			wireGuard: failSet{"Reconcile": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "rotate then access reconcile fails", method: http.MethodPost, path: "/api/v2/admin/players/1/wireguard/rotate",
			controller: failSet{"ReconcileAccessPolicies": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "revoke then gateway fails", method: http.MethodPost, path: "/api/v2/admin/players/1/wireguard/revoke",
			wireGuard: failSet{"Reconcile": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "gateway reconcile", method: http.MethodPost, path: "/api/v2/admin/wireguard/reconcile", want: http.StatusOK},
		{name: "gateway teardown", method: http.MethodPost, path: "/api/v2/admin/wireguard/teardown", want: http.StatusNoContent},
		{name: "access reconcile", method: http.MethodPost, path: "/api/v2/admin/access/reconcile", want: http.StatusOK},
		{name: "access teardown", method: http.MethodPost, path: "/api/v2/admin/access/teardown", want: http.StatusNoContent},
		{name: "challenge delete fails and compensation fails", method: http.MethodDelete, path: "/api/v2/admin/challenges/3",
			store: failSet{"DeleteAdminChallenge": errStoreDown}, controller: failSet{"ReconcileDeployments": errUpstreamDown}, want: http.StatusInternalServerError},
		{name: "deletion blocked on unknown match state", method: http.MethodDelete, path: "/api/v2/admin/challenges/3",
			matchState: "warming", want: http.StatusBadGateway},
		{name: "challenge resume access fails", method: http.MethodPost, path: "/api/v2/admin/challenges/1/resume",
			controller: failSet{"ReconcileAccessPolicies": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "validate unknown challenge", method: http.MethodPost, path: "/api/v2/admin/challenges/999/validate", want: http.StatusBadRequest},
		{name: "deploy unknown challenge", method: http.MethodPost, path: "/api/v2/admin/challenges/999/deploy", want: http.StatusBadRequest},
		{name: "deploy invalid package without message", method: http.MethodPost, path: "/api/v2/admin/challenges/1/deploy",
			validation: ChallengeValidationResult{ChallengeID: 1, Status: "invalid"}, want: http.StatusBadRequest},
		{name: "match start access fails", method: http.MethodPost, path: "/api/v2/admin/game/match/start",
			controller: failSet{"ReconcileAccessPolicies": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "match start game-core disabled", method: http.MethodPost, path: "/api/v2/admin/game/match/start",
			game: failSet{"StartMatch": errGameCoreDisabled}, want: http.StatusServiceUnavailable},
		{name: "match pause", method: http.MethodPost, path: "/api/v2/admin/game/match/pause", want: http.StatusOK},
		{name: "match resume", method: http.MethodPost, path: "/api/v2/admin/game/match/resume", want: http.StatusOK},
		{name: "match resume access fails", method: http.MethodPost, path: "/api/v2/admin/game/match/resume",
			controller: failSet{"ReconcileAccessPolicies": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "schedule bad body", method: http.MethodPut, path: "/api/v2/admin/game/match/schedule", body: "nope", want: http.StatusBadRequest},
		{name: "scheduler bad body", method: http.MethodPut, path: "/api/v2/admin/game/scheduler/interval", body: "nope", want: http.StatusBadRequest},
		{name: "scheduler update", method: http.MethodPut, path: "/api/v2/admin/game/scheduler/interval", body: UpdateSchedulerRequest{IntervalSeconds: 30}, want: http.StatusOK},
		{name: "scoreboard via scoring", method: http.MethodGet, path: "/api/v2/admin/game/scoreboard", want: http.StatusOK},
		{name: "scoreboard via failing game-core", method: http.MethodGet, path: "/api/v2/admin/game/scoreboard",
			scoring: disabledScoring, game: failSet{"Scoreboard": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "freeze bad body", method: http.MethodPost, path: "/api/v2/admin/game/scoreboard/freeze", body: "nope", want: http.StatusBadRequest},
		{name: "freeze bad unfreeze", method: http.MethodPost, path: "/api/v2/admin/game/scoreboard/freeze",
			body: scoreboardFreezeRequest{FreezeAt: "2099-01-01T00:00:00Z", UnfreezeAt: "soon"}, want: http.StatusBadRequest},
		{name: "freeze unfreeze before freeze", method: http.MethodPost, path: "/api/v2/admin/game/scoreboard/freeze",
			body: scoreboardFreezeRequest{FreezeAt: "2099-01-01T00:00:00Z", UnfreezeAt: "2098-01-01T00:00:00Z"}, want: http.StatusBadRequest},
		{name: "recompute via scoring", method: http.MethodPost, path: "/api/v2/admin/game/scoring/recompute", want: http.StatusOK},
		{name: "recompute via failing game-core", method: http.MethodPost, path: "/api/v2/admin/game/scoring/recompute",
			scoring: disabledScoring, game: failSet{"RecomputeScoring": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "audit via scoring", method: http.MethodGet, path: "/api/v2/admin/game/scoring/audit", want: http.StatusOK},
		{name: "audit via failing game-core", method: http.MethodGet, path: "/api/v2/admin/game/scoring/audit",
			scoring: disabledScoring, game: failSet{"AuditScoring": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "settings bad body", method: http.MethodPut, path: "/api/v2/admin/platform/settings", body: "nope", want: http.StatusBadRequest},
		{name: "appearance bad body", method: http.MethodPut, path: "/api/v2/admin/platform/appearance", body: "nope", want: http.StatusBadRequest},
		{name: "flag format reload", method: http.MethodPost, path: "/api/v2/admin/platform/settings/reload", want: http.StatusOK},
		{name: "flag format reload rejected", method: http.MethodPost, path: "/api/v2/admin/platform/settings/reload",
			game: failSet{"RefreshFlagFormat": errUpstreamDown}, want: http.StatusBadGateway},
		{name: "flag format reload not stored", method: http.MethodPost, path: "/api/v2/admin/platform/settings/reload",
			store: failSet{"SetActiveFlagFormat": errStoreDown}, want: http.StatusInternalServerError},
		{name: "checker run query clamps", method: http.MethodGet, path: "/api/v2/admin/game/checker-runs?limit=abc&offset=-4&tick_id=999999", want: http.StatusOK},
		{name: "checker run query caps", method: http.MethodGet, path: "/api/v2/admin/game/checker-runs?limit=9999&offset=x", want: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewMemoryStore(101)
			matchState := tc.matchState
			if matchState == "" {
				matchState = "not_started"
			}
			mux := httpapi.NewBaseMux(httpapi.ServiceInfo{Name: "api-gateway", Version: "test", Addr: ":0"})
			NewWithDeps("dev-team-token", "dev-admin-token", 101,
				selStore{inner: store, fail: tc.store},
				selController{inner: testControllerClient{validationResult: tc.validation, accessStatus: ControllerAccessStatus{State: "ready", Mode: "enforce"}}, fail: tc.controller},
				selWireGuard{inner: testWireGuardClient{status: WireGuardGatewayStatus{State: "ready", Mode: "wg"}}, fail: tc.wireGuard},
				selGameCore{inner: testGameCoreClient{match: GameMatchStatus{State: matchState}}, fail: tc.game},
			).WithScoringClient(selScoring{inner: testScoringClient{}, fail: tc.scoring}).RegisterRoutes(mux)
			env := &sweepEnv{t: t, mux: mux}
			if response := env.do(tc.method, tc.path, "dev-admin-token", tc.body); response.Code != tc.want {
				t.Fatalf("want %d, got %d %s", tc.want, response.Code, response.Body.String())
			}
		})
	}
}
