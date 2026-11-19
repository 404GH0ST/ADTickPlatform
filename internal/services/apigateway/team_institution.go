package apigateway

import (
	"context"
	"log"
	"strings"
	"unicode/utf8"
)

// maxTeamInstitutionLength bounds the display-only institution label so a
// long value cannot blow out the scoreboard, globe labels or first-blood banner.
const maxTeamInstitutionLength = 120

// normalizeTeamInstitution trims the institution and collapses internal runs of
// whitespace. ok is false when the value is too long or carries control
// characters.
func normalizeTeamInstitution(raw string) (string, bool) {
	value := strings.Join(strings.Fields(raw), " ")
	if utf8.RuneCountInString(value) > maxTeamInstitutionLength {
		return "", false
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	return value, true
}

// storedTeamInstitution is the value a store persists. Handlers and the bulk
// importer reject invalid input first, so this only normalizes whitespace and,
// defensively, clamps anything that slipped through.
func storedTeamInstitution(raw string) string {
	if value, ok := normalizeTeamInstitution(raw); ok {
		return value
	}
	runes := []rune(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.Join(strings.Fields(raw), " ")))
	if len(runes) > maxTeamInstitutionLength {
		runes = runes[:maxTeamInstitutionLength]
	}
	return strings.TrimSpace(string(runes))
}

// teamInstitutionsByName maps team name to institution for the read-side
// enrichment of scoreboard rows and attack events. Upstream scorers and the
// submission service only know team names, so api-gateway joins this display
// metadata in on the way out. It is best effort: a lookup failure leaves the
// payload without institutions instead of failing the read.
func (s *Server) teamInstitutionsByName(ctx context.Context) map[string]string {
	teams, err := s.store.ListAdminTeams(ctx)
	if err != nil {
		log.Printf("team institution lookup failed: %v", err)
		return nil
	}
	institutions := make(map[string]string, len(teams))
	for _, team := range teams {
		if team.Institution != "" {
			institutions[team.Name] = team.Institution
		}
	}
	return institutions
}

// withScoreInstitutions returns a copy of rows with Institution filled in. The
// input slice is never mutated: it may be a cached freeze snapshot.
func withScoreInstitutions(rows []scoreRow, institutions map[string]string) []scoreRow {
	if rows == nil {
		return nil
	}
	out := make([]scoreRow, len(rows))
	for index, row := range rows {
		row.Institution = institutions[row.Team]
		out[index] = row
	}
	return out
}

// withAttackInstitutions returns a copy of events with both institutions set.
func withAttackInstitutions(events []attackEvent, institutions map[string]string) []attackEvent {
	if events == nil {
		return nil
	}
	out := make([]attackEvent, len(events))
	for index, event := range events {
		event.AttackerInstitution = institutions[event.Attacker]
		event.VictimInstitution = institutions[event.Victim]
		out[index] = event
	}
	return out
}

func (s *Server) enrichScoreRows(ctx context.Context, rows []scoreRow) []scoreRow {
	return withScoreInstitutions(rows, s.teamInstitutionsByName(ctx))
}

func (s *Server) enrichAttackFeedPage(ctx context.Context, page AttackFeedPage) AttackFeedPage {
	page.Items = withAttackInstitutions(page.Items, s.teamInstitutionsByName(ctx))
	return page
}
