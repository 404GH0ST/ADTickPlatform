import { resolveThemeId } from "@/lib/themes";

/*
 * Server-side read of the organizer's theme choice. The root layout calls this
 * on every render, so the value is cached in-process for a few seconds and a
 * slow or unreachable gateway falls back to the last known (or default) theme
 * instead of blocking the page, including the login page.
 */

const CACHE_TTL_MS = (() => {
  const configured = Number(process.env.AD_PLATFORM_THEME_CACHE_MS);
  return Number.isFinite(configured) && configured >= 0 ? configured : 5_000;
})();
const FETCH_TIMEOUT_MS = 1_500;

let cached: { themeId: string; expiresAt: number } | null = null;

function apiBaseUrl() {
  return (process.env.AD_PLATFORM_API_URL ?? "http://127.0.0.1:8080").trim().replace(/\/+$/, "");
}

export async function getActiveThemeId(): Promise<string> {
  const now = Date.now();
  if (cached && cached.expiresAt > now) {
    return cached.themeId;
  }
  try {
    const response = await fetch(`${apiBaseUrl()}/api/v2/platform/appearance`, {
      cache: "no-store",
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
    if (!response.ok) {
      throw new Error(`appearance request failed with status ${response.status}`);
    }
    const body = (await response.json()) as { theme_id?: string };
    const themeId = resolveThemeId(body.theme_id);
    cached = { themeId, expiresAt: now + CACHE_TTL_MS };
    return themeId;
  } catch {
    // Keep serving the stale value briefly rather than hammering a down gateway.
    const themeId = cached?.themeId ?? resolveThemeId(null);
    cached = { themeId, expiresAt: now + CACHE_TTL_MS };
    return themeId;
  }
}

/** Called after the organizer saves, so this process reflects the change at once. */
export function setCachedActiveThemeId(themeId: string) {
  cached = { themeId: resolveThemeId(themeId), expiresAt: Date.now() + CACHE_TTL_MS };
}
