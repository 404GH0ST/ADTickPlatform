"use client";

import { Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";

type ThemeMode = "light" | "dark";

const storageKey = "ad-platform-theme";

function applyTheme(theme: ThemeMode) {
  document.documentElement.dataset.theme = theme;
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<ThemeMode>("light");
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const current =
      document.documentElement.dataset.theme === "dark" ? "dark" : "light";
    setTheme(current);
    setReady(true);
  }, []);

  function toggleTheme() {
    const nextTheme = theme === "dark" ? "light" : "dark";
    setTheme(nextTheme);
    applyTheme(nextTheme);
    try {
      window.localStorage.setItem(storageKey, nextTheme);
    } catch {}
  }

  return (
    <Button
      type="button"
      variant="outline"
      size="default"
      onClick={toggleTheme}
      aria-label={ready ? `Switch to ${theme === "dark" ? "light" : "dark"} theme` : "Toggle theme"}
      title={ready ? `Switch to ${theme === "dark" ? "light" : "dark"} theme` : "Toggle theme"}
    >
      {theme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
      <span>{theme === "dark" ? "Light" : "Dark"}</span>
    </Button>
  );
}
