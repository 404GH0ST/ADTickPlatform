// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from 'bun:test';

import type { AdminGameScoreRow } from './admin-dashboard-types';
import { computeCategoryLeaders } from './scoring-leaders';

function score(team: string, attack: number, defense: number, sla: number): AdminGameScoreRow {
  return { team, rank: 1, attack, defense, sla, total: attack + defense + sla, delta: '' };
}

test('award categories rank independently with stable ties and preserve the scoreboard', () => {
  const rows = [score('Zeta', 15, -2, 16), score('Alpha', 15, -6, 9), score('Bravo', 4, 0, 12)];
  expect(computeCategoryLeaders(rows, 2)).toEqual({
    attacker: [{ team: 'Alpha', score: 15, rank: 1 }, { team: 'Zeta', score: 15, rank: 2 }],
    defender: [{ team: 'Bravo', score: 0, rank: 1 }, { team: 'Zeta', score: -2, rank: 2 }],
    availability: [{ team: 'Zeta', score: 16, rank: 1 }, { team: 'Bravo', score: 12, rank: 2 }],
  });
  expect(rows.map(({ team }) => team)).toEqual(['Zeta', 'Alpha', 'Bravo']);
  expect(computeCategoryLeaders([...rows, score('Delta', 10, -2, 8)]).attacker).toHaveLength(3);
  expect(computeCategoryLeaders(rows, -1)).toEqual({ attacker: [], defender: [], availability: [] });
});
