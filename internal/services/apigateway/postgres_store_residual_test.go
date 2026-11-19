package apigateway

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestPostgresStoreEmptyAndMissingRows(t *testing.T) {
	s := postgresContractStore(t)
	ctx := context.Background()
	now := time.Now()
	w := buildWorld(t, s)
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := s.db.Exec(statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	store := s.Store.(*postgresStore)

	exec(`DELETE FROM game_match_state`)
	if paused, err := s.IsMatchPaused(ctx); err != nil || paused {
		t.Errorf("no match row means not paused: %v %v", paused, err)
	}
	s.setMatch(true, false)

	exec(`DELETE FROM platform_settings`)
	if settings, err := s.GetPlatformSettings(ctx); err != nil || settings.FlagFormatPrefix != "PLAYIT" {
		t.Errorf("default settings: %+v %v", settings, err)
	}
	if appearance, err := s.GetPlatformAppearance(ctx); err != nil || appearance.ThemeID != defaultPlatformThemeID {
		t.Errorf("default appearance: %+v %v", appearance, err)
	}
	solo, err := s.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Solo", Email: "pg-solo@example.test", Password: "contract-password"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.JoinExistingPlayerTeam(ctx, solo.PlayerID, w.teamB.JoinKey, now); err != nil {
		t.Errorf("with no settings row there is no member limit: %v", err)
	}
	if _, err := s.SetActiveFlagFormat(ctx, " ", now); err == nil {
		t.Error("an empty active format is refused")
	}

	exec(`DELETE FROM scoreboard_freeze`)
	if window, err := s.GetScoreboardFreeze(ctx); err != nil || window.FreezeAt != nil {
		t.Errorf("no freeze row means no freeze: %+v %v", window, err)
	}
	exec(`INSERT INTO scoreboard_freeze (id, freeze_at, snapshot) VALUES (1, NOW(), '[1]'::jsonb)`)
	if _, err := s.GetScoreboardFreeze(ctx); err == nil {
		t.Error("an unreadable snapshot is an error")
	}
	exec(`DELETE FROM scoreboard_freeze`)
	if _, err := s.SaveFrozenScoreboardSnapshot(ctx, []scoreRow{{Team: "x", Total: math.NaN()}}, now); err == nil {
		t.Error("a non-finite score cannot be stored")
	}

	exec(`INSERT INTO attack_events (id, attacker, victim, service, tick, verdict) VALUES (1, 'Contract A', 'Contract B', 'contract-web', 1, 'accepted')`)
	if events, err := s.ListAttackFeed(ctx); err != nil || len(events) != 1 {
		t.Errorf("attack feed rows: %+v %v", events, err)
	}
	if _, err := s.SubmitFlags(ctx, w.teamA.ID, []string{"x"}); !errors.Is(err, ErrSubmissionUnavailable) {
		t.Errorf("the gateway store never judges flags: %v", err)
	}

	// Unpublished, or without an instance, a service cannot be acted on.
	exec(`UPDATE challenges SET published = FALSE WHERE id = $1`, w.challenge.ID)
	expectErr(t, "an unpublished challenge", s.ValidateServiceAction(ctx, w.teamA.ID, w.challenge.ID), ErrChallengeNotFound)
	exec(`UPDATE challenges SET published = TRUE WHERE id = $1`, w.challenge.ID)
	exec(`DELETE FROM service_instances WHERE team_id = $1 AND challenge_id = $2`, w.teamB.ID, w.challenge.ID)
	expectErr(t, "a service with no instance", s.ValidateServiceAction(ctx, w.teamB.ID, w.challenge.ID), ErrChallengeNotFound)

	_, err = s.TransferTeamCaptain(ctx, 0, w.member.ID)
	expectErr(t, "a transfer without a captain ID", err, ErrInvalidCaptainTransfer)
	exec(`UPDATE players SET role = 'organizer' WHERE id = $1`, w.member.ID)
	_, err = s.TransferTeamCaptain(ctx, w.captain.ID, w.member.ID)
	expectErr(t, "captaincy to an organizer", err, ErrInvalidCaptainTransfer)
	exec(`UPDATE players SET role = 'member' WHERE id = $1`, w.member.ID)

	_, err = s.RotateAdminPlayerWireGuardConfig(ctx, mustRegister(t, s, "pg-nopeer@example.test").PlayerID, now)
	expectErr(t, "rotating a teamless player's missing peer", err, ErrPlayerNotFound)
	if _, err := s.db.Exec(`DELETE FROM wireguard_peers WHERE player_id = $1`, w.other.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.RotateAdminPlayerWireGuardConfig(ctx, w.other.ID, now)
	expectErr(t, "rotating a peer that was never issued", err, ErrPlayerNotFound)
	if _, err := s.RevokeAdminPlayerWireGuardConfig(ctx, w.member.ID, now); err != nil {
		t.Fatal(err)
	}
	if peer, err := s.GetAdminPlayerWireGuardConfig(ctx, w.member.ID); err != nil || peer.RevokedAt == "" {
		t.Errorf("a revoked peer reports when: %+v %v", peer, err)
	}
	if name, err := store.lookupTeamName(ctx, 0); err != nil || name != "Organizer" {
		t.Errorf("team 0 is the organizer team: %q %v", name, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustRegister(t *testing.T, s contractStore, email string) authenticatedPlayer {
	t.Helper()
	player, err := s.RegisterPlayer(context.Background(), participantRegisterRequest{DisplayName: "R", Email: email, Password: "contract-password"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return player
}

func TestPostgresStoreWithoutServerKeyOrAddresses(t *testing.T) {
	s := postgresContractStore(t)
	ctx := context.Background()
	now := time.Now()
	w := buildWorld(t, s)
	solo := mustRegister(t, s, "pg-keyless@example.test")
	missing, err := s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamB.ID, DisplayName: "Gap", Email: "pg-gap@example.test", Password: "contract-password"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM wireguard_peers WHERE player_id = $1`, missing.ID); err != nil {
		t.Fatal(err)
	}

	clearWireGuardEnv(t)
	t.Setenv("API_GATEWAY_STATE_BACKEND", "postgres")
	if _, err := s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamB.ID, DisplayName: "K", Email: "pg-k@example.test", Password: "contract-password"}, now); err == nil {
		t.Error("creating a team player needs a server key")
	}
	if _, err := s.JoinExistingPlayerTeam(ctx, solo.PlayerID, w.teamB.JoinKey, now); err == nil {
		t.Error("joining needs a server key")
	}
	if _, err := s.RotateAdminPlayerWireGuardConfig(ctx, w.captain.ID, now); err == nil {
		t.Error("rotating needs a server key")
	}
	if _, err := s.ListWireGuardGatewayPeers(ctx); err == nil {
		t.Error("repairing a missing peer needs a server key")
	}
	if _, err := s.GetAdminPlayerWireGuardConfig(ctx, w.captain.ID); err == nil {
		t.Error("refreshing an issued peer needs a server key")
	}
	t.Setenv("WIREGUARD_SERVER_PUBLIC_KEY", "contract-server-public-key")

	// Every team slot and peer address taken.
	if _, err := s.db.Exec(`INSERT INTO teams (id, name, email, join_key)
		SELECT g, 'Filler ' || g, 'filler-' || g || '@example.test', 'FILL-' || g
		FROM generate_series(101, 344) AS g ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateAdminTeam(ctx, adminCreateTeamRequest{Name: "Overflow", ContactEmail: "overflow@example.test"})
	expectErr(t, "team slot pool", err, ErrTeamAddressPool)
	if _, err := s.db.Exec(`
		WITH fillers AS (
			INSERT INTO players (display_name, email, password_hash, wireguard_peer)
			SELECT 'F' || g, 'fill-' || g || '@example.test', 'x', 'fill-' || g
			FROM generate_series(0, $1 - 1) AS g
			RETURNING id, wireguard_peer
		)
		INSERT INTO wireguard_peers (player_id, address, private_key, public_key, preshared_key, server_public_key, server_endpoint, dns, allowed_ips, config, issued_at, updated_at)
		SELECT f.id, '10.70.' || ((g / 200) + 1) || '.' || ((g % 200) + 20), 'k', 'k', 'k', 'k', 'e', 'd', 'a', 'c', NOW(), NOW()
		FROM fillers f JOIN LATERAL (SELECT substring(f.wireguard_peer FROM 6)::int AS g) n ON TRUE
		ON CONFLICT (address) DO NOTHING`, wireGuardPeerAddressPoolSize); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamB.ID, DisplayName: "Full", Email: "pg-full@example.test", Password: "contract-password"}, now)
	expectErr(t, "peer address pool", err, ErrPlayerAddressPool)
}
