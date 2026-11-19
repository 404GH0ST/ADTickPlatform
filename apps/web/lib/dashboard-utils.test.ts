// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from 'bun:test';
import {
  buildAttackFilterOptions,
  computePageOffset,
  handleRealtimeAttackMessage,
  isDefaultAttackFilters,
  parseNonNegativeInteger,
  parsePositiveInteger,
} from './dashboard-utils';

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

test('attack filters remain selectable and pagination honors bounds', () => {
  const defaults = { limit: '12', offset: '0', attacker: '', victim: '', service: '', tickFrom: '', tickTo: '' };
  expect(isDefaultAttackFilters({ ...defaults }, defaults)).toBe(true);
  expect(isDefaultAttackFilters({ ...defaults, service: 'Mail' }, defaults)).toBe(false);
  expect(buildAttackFilterOptions([' Zulu ', '', 'Alpha', 'Zulu'], ' Bravo ')).toEqual(['Alpha', 'Bravo', 'Zulu']);
  expect(buildAttackFilterOptions([])).toEqual([]);

  expect(parsePositiveInteger(null, 12)).toBe(12);
  expect(parsePositiveInteger('0', 12)).toBe(12);
  expect(parsePositiveInteger('invalid')).toBeUndefined();
  expect(parseNonNegativeInteger(null)).toBeUndefined();
  expect(parseNonNegativeInteger('-1')).toBeUndefined();
  expect(parseNonNegativeInteger('0')).toBe(0);
  expect(computePageOffset({ ...defaults, limit: '999' }, 'next').offset).toBe('200');
  expect(computePageOffset({ ...defaults, offset: '7' }, 'prev').offset).toBe('0');
  expect(computePageOffset({ ...defaults, limit: '', offset: '-3' }, 'next').offset).toBe('12');
});

test('short live snapshots preserve a longer page and skip invalid updates', () => {
  let page = { items: [{ id: 'two' }, { id: 'one' }], limit: 3, offset: 0, total_count: 2, has_prev: false, has_next: false };
  let live = false;
  let updates = 0;
  const handler = handleRealtimeAttackMessage({
    isLive: () => live,
    currentPage: () => page,
    limitStr: '3',
    setPage: (next) => { page = next; updates += 1; },
    scheduleHighlights: () => {},
  });
  const frame = new MessageEvent('message', { data: JSON.stringify({
    ...page, items: [{ id: 'three' }, { id: 'two' }], limit: 2, total_count: 4,
  }) });
  handler(frame);
  live = true;
  handler(new MessageEvent('message', { data: 'bad response' }));
  expect(updates).toBe(0);
  handler(frame);
  expect(page.items.map(({ id }) => id)).toEqual(['three', 'two', 'one']);
  expect(page.has_next).toBe(true);
  expect(page.limit).toBe(3);
});
