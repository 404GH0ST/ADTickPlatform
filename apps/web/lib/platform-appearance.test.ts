// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { expect, test } from "bun:test";

test("the theme survives a restart during an outage, and the API's answer wins", async () => {
  const originalFetch = globalThis.fetch;
  const originalTtl = process.env.AD_PLATFORM_THEME_CACHE_MS;
  // No caching, so every call below reaches the stubbed API. The module keeps
  // state in memory, so this runs as one sequence from a just-started server.
  process.env.AD_PLATFORM_THEME_CACHE_MS = "0";
  const apiDown = (async () => {
    throw new TypeError("fetch failed");
  }) as unknown as typeof fetch;

  try {
    const { getActiveTheme } = await import("./platform-appearance");

    globalThis.fetch = apiDown;
    expect(await getActiveTheme("neubrutalism")).toEqual({ themeId: "neubrutalism", confirmed: false });
    expect(await getActiveTheme(undefined)).toEqual({ themeId: "graphite", confirmed: false });
    expect(await getActiveTheme("not-a-theme")).toEqual({ themeId: "graphite", confirmed: false });

    globalThis.fetch = (async () => Response.json({ theme_id: "paper" })) as unknown as typeof fetch;
    expect(await getActiveTheme("neubrutalism")).toEqual({ themeId: "paper", confirmed: true });

    // Once this process has an answer, it outranks an older value in a browser.
    globalThis.fetch = apiDown;
    expect(await getActiveTheme("neubrutalism")).toEqual({ themeId: "paper", confirmed: true });
  } finally {
    globalThis.fetch = originalFetch;
    if (originalTtl === undefined) delete process.env.AD_PLATFORM_THEME_CACHE_MS;
    else process.env.AD_PLATFORM_THEME_CACHE_MS = originalTtl;
  }
});
