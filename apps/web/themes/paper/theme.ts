import { defineTheme } from "@/lib/theme-contract";

export default defineTheme({
  id: "paper",
  label: "Paper",
  description: "Warm workroom desk",
  swatch: {
    light: "oklch(0.975 0.012 75)",
    dark: "oklch(0.15 0.015 60)",
    accent: "oklch(0.42 0.08 55)",
  },
});
