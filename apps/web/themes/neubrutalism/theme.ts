import { defineTheme } from "@/components/theme/kit";

import { NbFlagSubmit } from "./components/nb-flag-submit";
import { NbLogin } from "./components/nb-login";
import { NbOrganizerScoreboard } from "./components/nb-organizer-scoreboard";
import { NbOverview } from "./components/nb-overview";
import { NbScoreboard } from "./components/nb-scoreboard";
import { NbServices } from "./components/nb-services";
import { NbShell } from "./components/nb-shell";

/**
 * Neubrutalism: white grid paper, black outlines, hard offset shadows, and
 * electric blue chrome with sunshine yellow and aqua fills. It replaces the shell (both surfaces), the participant home,
 * scoreboard, services, flag form and login, and the organizer scoreboard.
 * Every other view (attacks, dialogs, docs, the rest of the organizer
 * console) keeps its stock layout and is restyled through the primitive hooks
 * in theme.css, so the platform reads as one design.
 */
export default defineTheme({
  id: "neubrutalism",
  label: "Neubrutalism",
  description: "Blue chrome, yellow and aqua blocks, hard shadows",
  swatch: {
    light: "oklch(0.995 0.002 265)",
    dark: "oklch(0.175 0.012 265)",
    accent: "oklch(0.565 0.225 263)",
  },
  defaultMode: "light",
  components: {
    AppShell: NbShell,
    Overview: NbOverview,
    Scoreboard: NbScoreboard,
    Services: NbServices,
    FlagSubmit: NbFlagSubmit,
    Login: NbLogin,
    OrganizerScoreboard: NbOrganizerScoreboard,
  },
});
