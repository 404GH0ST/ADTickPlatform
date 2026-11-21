import { resolveThemeId } from "@/lib/themes";

/*
 * Server-side read of the organizer's theme choice. The root layout calls this
 * on every render, so the value is cached in-process for a few seconds and a
 * slow or unreachable gateway falls back instead of blocking the page,
 * including the login page.
 *
 * This process's memory is empty after a restart, so the browser keeps the last
 * confirmed theme in a cookie and the layout passes it in as the fallback.
 */

const CACHE_TTL_MS = (() => {
  const configured = Number(process.env.AD_PLATFORM_THEME_CACHE_MS);
  return Number.isFinite(configured) && configured >= 0 ? configured : 5_000;
})();
const FETCH_TIMEOUT_MS = 1_500;

let confirmed: { themeId: string; expiresAt: number } | null = null;
// With nothing confirmed yet, avoid hammering a down gateway on every render.
let retryAt = 0;

export type ActiveTheme = {
  themeId: string;
  /** False when the API has not confirmed this theme since the server started. */
  confirmed: boolean;
};

function apiBaseUrl() {
  return (process.env.AD_PLATFORM_API_URL ?? "http://127.0.0.1:8080").trim().replace(/\/+$/, "");
}

export async function getActiveTheme(lastKnownThemeId?: string): Promise<ActiveTheme> {
  const now = Date.now();
  if (confirmed && confirmed.expiresAt > now) {
    return { themeId: confirmed.themeId, confirmed: true };
  }
  if (!confirmed && retryAt > now) {
    return { themeId: resolveThemeId(lastKnownThemeId), confirmed: false };
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
    confirmed = { themeId, expiresAt: now + CACHE_TTL_MS };
    return { themeId, confirmed: true };
  } catch {
    if (confirmed) {
      confirmed = { themeId: confirmed.themeId, expiresAt: now + CACHE_TTL_MS };
      return { themeId: confirmed.themeId, confirmed: true };
    }
    retryAt = now + CACHE_TTL_MS;
    return { themeId: resolveThemeId(lastKnownThemeId), confirmed: false };
  }
}

/** Called after the organizer saves, so this process reflects the change at once. */
export function setCachedActiveThemeId(themeId: string) {
  confirmed = { themeId: resolveThemeId(themeId), expiresAt: Date.now() + CACHE_TTL_MS };
}
