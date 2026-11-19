import dynamic from "next/dynamic";

import type { ThemeComponents } from "@/lib/theme-contract";

export default {
  AppShell: dynamic(() => import("./components/nb-shell").then((module) => module.NbShell)),
  Overview: dynamic(() => import("./components/nb-overview").then((module) => module.NbOverview)),
  Scoreboard: dynamic(() => import("./components/nb-scoreboard").then((module) => module.NbScoreboard)),
  Services: dynamic(() => import("./components/nb-services").then((module) => module.NbServices)),
  FlagSubmit: dynamic(() => import("./components/nb-flag-submit").then((module) => module.NbFlagSubmit)),
  Login: dynamic(() => import("./components/nb-login").then((module) => module.NbLogin)),
  OrganizerScoreboard: dynamic(() => import("./components/nb-organizer-scoreboard").then((module) => module.NbOrganizerScoreboard)),
} satisfies ThemeComponents;
