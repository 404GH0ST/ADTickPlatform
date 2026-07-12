type BaseAttackFeedPage<T extends { id: string }> = {
  items: T[];
  limit: number;
  offset: number;
  total_count: number;
  has_prev: boolean;
  has_next: boolean;
};

export function parsePositiveInteger(raw: string | null, fallback?: number, max = 0): number | undefined {
  if (!raw) {
    return fallback;
  }
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  if (max > 0 && parsed > max) {
    return max;
  }
  return parsed;
}

export function parseNonNegativeInteger(raw: string | null): number | undefined {
  if (!raw) {
    return undefined;
  }
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return undefined;
  }
  return parsed;
}

export type AttackFilters = {
  limit: string;
  offset: string;
  attacker: string;
  victim: string;
  service: string;
  tickFrom: string;
  tickTo: string;
};

export function isDefaultAttackFilters(
  filters: AttackFilters,
  defaults: AttackFilters,
): boolean {
  return (
    filters.limit === defaults.limit &&
    filters.offset === defaults.offset &&
    filters.attacker === defaults.attacker &&
    filters.victim === defaults.victim &&
    filters.service === defaults.service &&
    filters.tickFrom === defaults.tickFrom &&
    filters.tickTo === defaults.tickTo
  );
}

/**
 * Unique, sorted option labels for attack filter dropdowns.
 * Always includes the current filter value so deep-linked filters stay selectable.
 */
export function buildAttackFilterOptions(
  candidates: Iterable<string>,
  current = "",
): string[] {
  const values = new Set<string>();
  for (const candidate of candidates) {
    const trimmed = candidate.trim();
    if (trimmed) {
      values.add(trimmed);
    }
  }
  const currentTrimmed = current.trim();
  if (currentTrimmed) {
    values.add(currentTrimmed);
  }
  return Array.from(values).sort((a, b) => a.localeCompare(b));
}

/**
 * Create an SSE `onmessage` handler for attack feed events.
 * Shared between organizer and participant dashboard hooks.
 */
export function handleRealtimeAttackMessage<T extends { id: string }>(opts: {
  isLive: () => boolean;
  currentPage: () => BaseAttackFeedPage<T>;
  limitStr: string;
  setPage: (page: BaseAttackFeedPage<T>) => void;
  scheduleHighlights: (ids: string[]) => void;
}): (event: MessageEvent) => void {
  return (event: MessageEvent) => {
    if (!opts.isLive()) return;
    try {
      const nextPage = JSON.parse(event.data) as BaseAttackFeedPage<T>;
      const mergedPage = mergeRealtimeAttackPage(
        opts.currentPage(),
        nextPage,
        opts.limitStr,
      );
      const nextHighlights = collectNewAttackIDs(opts.currentPage(), mergedPage);
      opts.setPage(mergedPage);
      if (nextHighlights.length > 0) {
        opts.scheduleHighlights(nextHighlights);
      }
    } catch {
      // Keep the last good snapshot if one frame is malformed.
    }
  };
}

/**
 * Compute the next page offset for attack/checker-run/scheduler paging.
 * Shared between organizer and participant hooks.
 */
export function computePageOffset(
  filters: AttackFilters,
  direction: "prev" | "next",
  defaultLimit = 12,
  maxLimit = 200,
): AttackFilters {
  const limit = parsePositiveInteger(filters.limit, defaultLimit, maxLimit) ?? defaultLimit;
  const currentOffset = parseNonNegativeInteger(filters.offset) ?? 0;
  const nextOffset =
    direction === "prev"
      ? Math.max(0, currentOffset - limit)
      : currentOffset + limit;
  return { ...filters, offset: String(nextOffset) };
}

function collectNewAttackIDs<T extends { id: string }>(currentPage: BaseAttackFeedPage<T>, nextPage: BaseAttackFeedPage<T>): string[] {
  const currentIDs = new Set(currentPage.items.map((item) => item.id));
  return nextPage.items
    .filter((item) => !currentIDs.has(item.id))
    .map((item) => item.id)
    .slice(0, 4);
}

function mergeRealtimeAttackPage<T extends { id: string }>(
  currentPage: BaseAttackFeedPage<T>,
  streamedPage: BaseAttackFeedPage<T>,
  requestedLimit: string,
): BaseAttackFeedPage<T> {
  const limit = parsePositiveInteger(requestedLimit, streamedPage.limit) ?? streamedPage.limit;
  if (limit <= streamedPage.limit) {
    return streamedPage;
  }

  const mergedItems = [...streamedPage.items];
  const seenIDs = new Set(mergedItems.map((item) => item.id));
  for (const item of currentPage.items) {
    if (seenIDs.has(item.id)) {
      continue;
    }
    mergedItems.push(item);
    seenIDs.add(item.id);
    if (mergedItems.length >= limit) {
      break;
    }
  }

  return {
    ...streamedPage,
    items: mergedItems,
    limit,
    has_next: streamedPage.total_count > mergedItems.length,
  };
}
