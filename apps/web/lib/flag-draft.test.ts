// @ts-expect-error Bun exposes its test module at runtime without bundled TypeScript types.
import { afterEach, beforeEach, expect, test } from "bun:test";

import { clearFlagDraft, readFlagDraft, writeFlagDraft } from "./flag-draft";

const globals = globalThis as unknown as { window?: unknown };
const originalWindow = globals.window;
let stored: Map<string, string>;

beforeEach(() => {
  stored = new Map();
  globals.window = {
    sessionStorage: {
      getItem: (key: string) => stored.get(key) ?? null,
      setItem: (key: string, value: string) => stored.set(key, value),
      removeItem: (key: string) => stored.delete(key),
    },
  };
});

afterEach(() => {
  globals.window = originalWindow;
});

test("a draft comes back for the player who wrote it", () => {
  writeFlagDraft("101:1001", "PLAYIT{mine}");
  expect(readFlagDraft("101:1001")).toBe("PLAYIT{mine}");
});

test("another account never sees a draft, and reading deletes it", () => {
  writeFlagDraft("101:1001", "PLAYIT{private}");
  expect(readFlagDraft("102:2002")).toBe("");
  expect(readFlagDraft("101:1001")).toBe("");
  expect(stored.size).toBe(0);
});

test("a draft from an older format is discarded", () => {
  stored.set("ad-platform-flag-draft", "PLAYIT{no_owner}");
  expect(readFlagDraft("101:1001")).toBe("");
  expect(stored.size).toBe(0);
});

test("clearing and empty text remove the draft", () => {
  writeFlagDraft("101:1001", "PLAYIT{x}");
  writeFlagDraft("101:1001", "");
  expect(stored.size).toBe(0);
  writeFlagDraft("101:1001", "PLAYIT{y}");
  clearFlagDraft();
  expect(stored.size).toBe(0);
});

test("blocked storage is harmless", () => {
  globals.window = {
    get sessionStorage() {
      throw new Error("blocked");
    },
  };
  writeFlagDraft("101:1001", "PLAYIT{x}");
  expect(readFlagDraft("101:1001")).toBe("");
  clearFlagDraft();
});
