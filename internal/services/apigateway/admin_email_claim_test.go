package apigateway

import (
	"context"
	"testing"
	"time"
)

func TestStoreContractOrganizerReplacesUnclaimedRegistration(t *testing.T) {
	forEachStore(t, func(t *testing.T, s contractStore) {
		ctx := context.Background()
		now := time.Now()
		w := buildWorld(t, s)

		squatter, err := s.RegisterPlayer(ctx, participantRegisterRequest{DisplayName: "Squatter", Email: "claimed@example.test", Password: "squatter-password"}, now)
		if err != nil {
			t.Fatal(err)
		}
		created, err := s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamB.ID, DisplayName: "Real Player", Email: "CLAIMED@example.test", Password: "organizer-password", Role: "member"}, now)
		if err != nil {
			t.Fatalf("an unclaimed registration must not block provisioning: %v", err)
		}
		if created.ID == squatter.PlayerID {
			t.Fatal("the provisioned account must be new")
		}
		_, err = s.AuthenticatePlayer(ctx, "claimed@example.test", "squatter-password")
		expectErr(t, "replaced password", err, ErrInvalidCredentials)
		if _, err := s.ValidatePlayerSession(ctx, squatter.PlayerID, 0, "member", squatter.SessionVersion); err == nil {
			t.Fatal("the replaced account's session must stop working")
		}
		player, err := s.AuthenticatePlayer(ctx, "claimed@example.test", "organizer-password")
		if err != nil || player.TeamID != w.teamB.ID {
			t.Fatalf("provisioned login: %+v %v", player, err)
		}

		_, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{TeamID: w.teamB.ID, DisplayName: "Again", Email: "claimed@example.test", Password: "another-password", Role: "member"}, now)
		expectErr(t, "account on a team", err, ErrDuplicateResource)

		organizer, err := s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{DisplayName: "Organizer", Email: "organizer-claim@example.test", Password: "organizer-password", Role: "organizer"}, now)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.CreateAdminPlayer(ctx, adminCreatePlayerRequest{DisplayName: "Replacement", Email: organizer.Email, Password: "replacement-password", Role: "member"}, now)
		expectErr(t, "organizer account", err, ErrDuplicateResource)
	})
}
