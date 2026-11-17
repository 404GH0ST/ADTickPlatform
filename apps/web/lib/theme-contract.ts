import type { ComponentProps, ComponentType } from "react";

import type { OrganizerAuditViewProps } from "@/components/admin/audit-log-view";
import type { OrganizerOverviewViewProps } from "@/components/admin/organizer-overview";
import type { AnnouncementsState } from "@/components/admin/use-announcements";
import type { TrustedState } from "@/components/admin/use-trusted-state";
import type { ApiDocsViewProps } from "@/components/docs/api-docs";
import type { ParticipantManualViewProps } from "@/components/docs/participant-manual";
import type {
  ChallengesTab,
  DeploymentsTab,
  GameAttacksCard,
  GameTab,
  PlayersTab,
  ScoreboardTab,
  TeamsTab,
} from "@/components/admin/organizer-dashboard-sections";
import type { SubmissionsTable } from "@/components/admin/submissions-table";

import type {
  AttacksPanelProps,
  FactoryResetDialogProps,
  ScoreboardPanelProps,
  ServicesPanelProps,
  SSHSessionDialogProps,
} from "@/components/dashboard/control-center-sections";
import type { FlagSubmitState } from "@/components/dashboard/use-flag-submit";
import type { ParticipantLoginState } from "@/components/dashboard/use-participant-login";
import type { AppShellProps } from "@/components/ui/default-app-shell";
import type { ThemePreference } from "@/lib/appearance";

/**
 * Contract for a theme folder under `themes/<id>/`.
 *
 * A theme is two files (plus optional extras):
 *   - `theme.ts`  default-exports `defineTheme({...})` (this type)
 *   - `theme.css` color tokens scoped to `:root[data-scheme="<id>"]` (light)
 *                 and `:root[data-scheme="<id>"][data-theme="dark"]` (dark)
 *
 * Tokens a theme does not set fall back to the default Graphite theme, so a
 * theme may override as little as a handful of colors.
 *
 * `components` lets a theme replace whole views. Every view receives the
 * same state and callbacks as the stock one (see ThemeSlotProps), so a theme
 * can redesign a page completely while the features behind it (live updates,
 * flag submission, SSH sessions, restart, factory reset, cooldowns, filters)
 * keep working unchanged. Overrides render inside client components, so they
 * must be client-safe: no server-only imports and no data fetching of their
 * own. If an override throws, the stock view renders instead.
 */
export type ThemeDefinition = {
  /** Must equal the folder name: lowercase slug (a-z, 0-9, -), ≤48 chars. */
  id: string;
  label: string;
  description: string;
  /** Preview chips for the organizer picker: light surface, dark surface, accent. */
  swatch: { light: string; dark: string; accent: string };
  /**
   * Mode used until a participant picks one themselves. Defaults to "system".
   * Participants can always switch light/dark/system afterwards.
   */
  defaultMode?: ThemePreference;
  components?: ThemeComponents;
};

/** Props each replaceable view receives. Keys are the slot names. */
export type ThemeSlotProps = {
  /** Page chrome on every participant and organizer page. */
  AppShell: AppShellProps;
  /** Participant home page body (the "/" route). */
  Overview: OverviewViewProps;
  /** Participant scoreboard page body. */
  Scoreboard: ScoreboardPanelProps;
  /** Participant services page body. `flagSubmit` is the themed flag form, ready to place. */
  Services: ServicesViewProps;
  /** Flag submission form. Bound to the shared submit logic (useFlagSubmit). */
  FlagSubmit: FlagSubmitState;
  /** Participant attacks page body (map, feed, filters). */
  Attacks: AttacksPanelProps;
  /** Confirmation / credential dialog for SSH sessions. */
  SSHSessionDialog: SSHSessionDialogProps;
  /** Confirmation dialog for service factory resets. */
  FactoryResetDialog: FactoryResetDialogProps;
  /** Login / registration form. Bound to the shared auth logic (useParticipantLogin). */
  Login: ParticipantLoginState;
  /** Participant manual (/docs/participant). */
  ParticipantManual: ParticipantManualViewProps;
  /** Participant API docs (/docs/platform-api); `explorer` is the Swagger UI. */
  ApiDocs: ApiDocsViewProps;

  // Organizer console. Each tab receives the state and actions of
  // useOrganizerDashboard() that the stock tab uses.
  /** /admin/teams body. */
  OrganizerTeams: ComponentProps<typeof TeamsTab>;
  /** /admin/players body. */
  OrganizerPlayers: ComponentProps<typeof PlayersTab>;
  /** /admin/challenges body. */
  OrganizerChallenges: ComponentProps<typeof ChallengesTab>;
  /** /admin/deployments body. */
  OrganizerDeployments: ComponentProps<typeof DeploymentsTab>;
  /** /admin/game body: match lifecycle, scheduler, runtime health, settings. */
  OrganizerGame: ComponentProps<typeof GameTab>;
  /** /admin/scoreboard body. */
  OrganizerScoreboard: ComponentProps<typeof ScoreboardTab>;
  /** /admin/attacks body. */
  OrganizerAttacks: ComponentProps<typeof GameAttacksCard>;
  /** /admin/submissions table (with delete). */
  OrganizerSubmissions: ComponentProps<typeof SubmissionsTable>;
  /** /admin/audit body: filters, log table, paging. */
  OrganizerAudit: OrganizerAuditViewProps;
  /** Organizer home (/admin): trusted state, announcements, section links. */
  OrganizerOverview: OrganizerOverviewViewProps;
  /** Match announcements panel (useAnnouncements). */
  OrganizerAnnouncements: AnnouncementsState;
  /** Trusted runtime state panel on /admin and /admin/deployments (useTrustedState). */
  OrganizerTrustedState: TrustedState;
};

export type ThemeSlotName = keyof ThemeSlotProps;

export type ThemeComponents = {
  [K in ThemeSlotName]?: ComponentType<ThemeSlotProps[K]>;
};

export type OverviewLink = {
  id: "scoreboard" | "services" | "attacks" | "docs";
  href: string;
  title: string;
  description: string;
  meta: string;
};

export type OverviewViewProps = {
  links: OverviewLink[];
  teamName?: string;
  scoreCount: number;
  serviceCount: number;
  attackCount: number;
};

export type ServicesViewProps = ServicesPanelProps & {
  /** Rendered flag form (themed via the FlagSubmit slot), or null when the viewer cannot submit. */
  flagSubmit: React.ReactNode | null;
};

/** Must match DEFAULT_THEME_ID in scripts/generate-themes.mjs and the gateway default. */
export const DEFAULT_THEME_ID = "graphite";

export const THEME_ID_PATTERN = /^[a-z0-9][a-z0-9-]{0,47}$/;

export function defineTheme<T extends ThemeDefinition>(theme: T): T {
  return theme;
}
