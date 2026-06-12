import type { AdminGameScoreRow } from "./admin-dashboard-types";

/**
 * One ranked team in a single scoring category.
 * `rank` is 1-based (1 = top of the category).
 */
export type CategoryLeader = {
  team: string;
  score: number;
  rank: number;
};

/**
 * Top-N leaders for the three award categories.
 * `attacker` and `availability` are sorted high-to-low.
 * `defender` is sorted high-to-low on the `defense` field,
 * which is itself a (usually non-positive) penalty score: the
 * team with the highest value is the best defender (least
 * penalty, or net positive if its services never got captured).
 */
export type CategoryLeadersSummary = {
  attacker: CategoryLeader[];
  defender: CategoryLeader[];
  availability: CategoryLeader[];
};

const DEFAULT_TOP_N = 3;

/**
 * Build the award-candidate leaders for the three scoring categories
 * from the authoritative, cumulative scoreboard.
 *
 * "Kumulatif" is enforced by the upstream scorer: attack, defense, and
 * SLA values in `AdminGameScoreRow` already include every accepted flag
 * capture, every applied defense penalty, and every recorded service
 * state for the whole match. This helper does not re-derive totals; it
 * only sorts and slices the rows game-core already produced.
 *
 * Ties are broken by team name ascending for a stable, deterministic
 * ranking — the same input always yields the same output, which the
 * e2e tests and any future awards API depend on.
 *
 * Pure function. Safe on the server (used by the RSC page) and on the
 * client (used by the live-updating dashboard).
 */
export function computeCategoryLeaders(
  scoreRows: AdminGameScoreRow[],
  topN: number = DEFAULT_TOP_N,
): CategoryLeadersSummary {
  const limit = Math.max(0, topN);

  const pick = (select: (row: AdminGameScoreRow) => number): CategoryLeader[] => {
    const sorted = [...scoreRows].sort((a, b) => {
      const diff = select(b) - select(a);
      if (diff !== 0) return diff;
      return a.team.localeCompare(b.team);
    });

    return sorted.slice(0, limit).map((row, index) => ({
      team: row.team,
      score: select(row),
      rank: index + 1,
    }));
  };

  return {
    attacker: pick((row) => row.attack),
    defender: pick((row) => row.defense),
    availability: pick((row) => row.sla),
  };
}
