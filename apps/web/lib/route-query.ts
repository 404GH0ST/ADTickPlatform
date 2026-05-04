import { parsePositiveInteger, parseNonNegativeInteger } from "@/lib/dashboard-utils";
export { parsePositiveInteger, parseNonNegativeInteger };


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
