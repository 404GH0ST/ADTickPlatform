package apigateway

import (
	"cmp"
	"slices"
)

// rankScoreRows orders rows in place, eligible teams first and disqualified
// teams last, each group by total, attack, defense, then name. It matches the
// game-core ranking so a re-ranked snapshot agrees with a live build.
func rankScoreRows(rows []scoreRow) {
	slices.SortStableFunc(rows, func(a, b scoreRow) int {
		if a.Disqualified != b.Disqualified {
			if a.Disqualified {
				return 1
			}
			return -1
		}
		return cmp.Or(
			cmp.Compare(b.Total, a.Total),
			cmp.Compare(b.Attack, a.Attack),
			cmp.Compare(b.Defense, a.Defense),
			cmp.Compare(a.Team, b.Team),
		)
	})
	for index := range rows {
		rows[index].Rank = index + 1
	}
}

// withCurrentDisqualifications applies today's team status to a frozen
// snapshot. Scores stay as frozen, but a disqualification or reinstatement is
// an organizer decision that takes effect on the public board immediately.
// The input slice is never mutated.
func withCurrentDisqualifications(rows []scoreRow, teams []adminTeam) []scoreRow {
	if rows == nil {
		return nil
	}
	byIdentity := make(map[string]adminTeam, len(teams))
	for _, team := range teams {
		if team.ScoreIdentity != "" {
			byIdentity[team.ScoreIdentity] = team
		}
	}
	out := slices.Clone(rows)
	for index := range out {
		team := byIdentity[out[index].TeamIdentity]
		out[index].Disqualified = !team.Active
		out[index].Institution = team.Institution
	}
	rankScoreRows(out)
	return out
}
