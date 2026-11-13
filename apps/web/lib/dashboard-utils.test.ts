// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from 'bun:test';
import { handleRealtimeAttackMessage } from './dashboard-utils';

test('first blood is notified beyond the normal sound cap and never repeated by the same snapshot', () => {
  type Event = { id: string; first_blood?: boolean };
  let page = { items: [{ id: 'known', first_blood: true }] as Event[], limit: 20, offset: 0, total_count: 1, has_prev: false, has_next: false };
  const calls: Array<[string[], string[]]> = [];
  const handler = handleRealtimeAttackMessage({
    isLive: () => true,
    currentPage: () => page,
    limitStr: '20',
    setPage: (next) => { page = next; },
    scheduleHighlights: (ids, milestones) => calls.push([ids, milestones]),
  });
  const items = [...Array.from({ length: 5 }, (_, i) => ({ id: `ordinary-${i}` })), { id: 'winner', first_blood: true }, ...page.items];
  const frame = { data: JSON.stringify({ ...page, items, total_count: items.length }) } as MessageEvent;
  handler(frame);
  handler(frame);
  expect(calls).toHaveLength(1);
  expect(calls[0][0]).toHaveLength(4);
  expect(calls[0][1]).toEqual(['winner']);
});

test('a capture deletion invalidates the full attack slice without replaying sounds', () => {
  let page = { items: [{ id: 'removed' }, { id: 'retained' }], limit: 1000, offset: 0, total_count: 2, has_prev: false, has_next: false, revision: 0 };
  let reloads = 0;
  let sounds = 0;
  const handler = handleRealtimeAttackMessage({
    isLive: () => true, currentPage: () => page, limitStr: '1000',
    setPage: (next) => { page = { ...next, revision: next.revision ?? 0 }; },
    scheduleHighlights: () => { sounds++; }, onInvalidate: () => { reloads++; },
  });
  handler({ data: JSON.stringify({ ...page, revision: 1, items: [{ id: 'retained' }], limit: 12, total_count: 1 }) } as MessageEvent);
  expect(page.items).toEqual([{ id: 'retained' }]);
  expect(reloads).toBe(1);
  expect(sounds).toBe(0);
});

test('a new first blood in the deletion frame still receives its live effects', () => {
  type Item = { id: string; first_blood?: boolean };
  let page = { items: [{ id: 'removed' }] as Item[], limit: 1000, offset: 0, total_count: 1, has_prev: false, has_next: false, revision: 0 };
  let milestones: string[] = [];
  const handler = handleRealtimeAttackMessage({
    isLive: () => true, currentPage: () => page, limitStr: '1000',
    setPage: (next) => { page = { ...next, revision: next.revision ?? 0 }; },
    scheduleHighlights: (_ids, firstBlood) => { milestones = firstBlood; },
  });
  handler({ data: JSON.stringify({ ...page, revision: 1, items: [{ id: 'next-winner', first_blood: true }], limit: 12 }) } as MessageEvent);
  expect(milestones).toEqual(['next-winner']);
  expect(page.items).toEqual([{ id: 'next-winner', first_blood: true }]);
});
