export function parsePositiveInteger(
  raw: string | null,
  fallback?: number,
  max = 0,
): number | undefined {
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

function parseTextFilter(raw: string | null): string | undefined {
  if (!raw) {
    return undefined;
  }

  const trimmed = raw.trim();
  return trimmed === "" ? undefined : trimmed;
}

export function parseLowercaseTextFilter(raw: string | null): string | undefined {
  return parseTextFilter(raw)?.toLowerCase();
}

export function parseAttackFeedQuery(
  searchParams: URLSearchParams,
  defaultLimit = 12,
  maxLimit = 200,
) {
  return {
    limit: parsePositiveInteger(searchParams.get("limit"), defaultLimit, maxLimit),
    offset: parseNonNegativeInteger(searchParams.get("offset")),
    attacker: parseTextFilter(searchParams.get("attacker")),
    victim: parseTextFilter(searchParams.get("victim")),
    service: parseTextFilter(searchParams.get("service")),
    tick_from: parseNonNegativeInteger(searchParams.get("tick_from")),
    tick_to: parseNonNegativeInteger(searchParams.get("tick_to")),
  };
}
