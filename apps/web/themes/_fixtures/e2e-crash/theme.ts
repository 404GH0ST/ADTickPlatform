import { defineTheme } from "@/lib/theme-contract";

/**
 * Test-only theme (registered only when AD_PLATFORM_THEME_FIXTURES=1).
 * Its views throw on purpose so e2e can prove a broken theme falls back to
 * the stock views on both the server render and the client.
 */
function crash(slot: string): never {
  throw new Error(`e2e-crash theme: ${slot} view exploded on purpose`);
}

export default defineTheme({
  id: "e2e-crash",
  label: "E2E Crash",
  description: "Test fixture: every view throws",
  swatch: { light: "#ffffff", dark: "#000000", accent: "#ff0000" },
  components: {
    Scoreboard: () => crash("Scoreboard"),
    Services: () => crash("Services"),
    OrganizerAudit: () => crash("OrganizerAudit"),
    OrganizerTrustedState: () => crash("OrganizerTrustedState"),
  },
});
