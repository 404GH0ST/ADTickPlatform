"use client";

import { useEffect } from "react";

import { rememberActiveTheme } from "@/lib/appearance";

/** Keeps the confirmed theme in the browser so a server restart during an outage can't drop it. */
export function RememberActiveTheme({ themeId }: { themeId: string }) {
  useEffect(() => {
    rememberActiveTheme(themeId);
  }, [themeId]);
  return null;
}
