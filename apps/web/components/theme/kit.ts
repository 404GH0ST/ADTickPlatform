/**
 * Theme kit: the supported import surface for theme folders.
 *
 *   import { Button, describeService, DefaultScoreboardView } from "@/components/theme/kit";
 *
 * Everything a themed view needs lives here: the stock views (to wrap or fall
 * back to), the logic hooks behind them, display helpers, UI primitives and
 * the data types. Prefer this module over deep imports so themes keep
 * working when internals move.
 */

// Contract
export { defineTheme } from "@/lib/theme-contract";
export type {
  OverviewLink,
  OverviewViewProps,
  ServicesViewProps,
  ThemeComponents,
  ThemeDefinition,
  ThemeSlotName,
  ThemeSlotProps,
} from "@/lib/theme-contract";
export { useActiveTheme } from "@/components/theme/active-theme-provider";
export { ThemeSlot } from "@/components/theme/theme-slot";

// Stock views, one per slot. Render them to wrap or partially reuse a view.
export { DefaultAppShell } from "@/components/ui/default-app-shell";
export type { AppShellProps, NavItem } from "@/components/ui/default-app-shell";
export { DefaultOverviewView } from "@/components/dashboard/participant-overview";
export { DefaultServicesView } from "@/components/dashboard/control-center";
export {
  AttacksPanel as DefaultAttacksView,
  FactoryResetDialog as DefaultFactoryResetDialog,
  ResetWarningBlock,
  ScoreboardPanel as DefaultScoreboardView,
  ServicesPanel as DefaultServiceCards,
  SSHSessionDialog as DefaultSSHSessionDialog,
  TickIntervalCard as MatchClockCard,
  useMatchClock,
} from "@/components/dashboard/control-center-sections";
export type {
  AttackFilters,
  AttacksPanelProps as AttacksViewProps,
  FactoryResetDialogProps,
  ScoreboardPanelProps as ScoreboardViewProps,
  SSHSessionData,
  SSHSessionDialogProps,
  MatchClock,
} from "@/components/dashboard/control-center-sections";
export { DefaultFlagSubmitView, FlagSubmitPanel } from "@/components/dashboard/flag-submit-panel";
export { DefaultLoginView } from "@/components/dashboard/participant-login-form";
export {
  ChallengesTab as DefaultOrganizerChallenges,
  DeploymentsTab as DefaultOrganizerDeployments,
  GameAttacksCard as DefaultOrganizerAttacks,
  GameTab as DefaultOrganizerGame,
  PlayersTab as DefaultOrganizerPlayers,
  ScoreboardTab as DefaultOrganizerScoreboard,
  // Scoreboard tab building blocks, for themes that redraw only the standings.
  ScoreboardCategoryLeadersPanel,
  ScoreboardFreezeControl,
  ScoringAuditCard,
  TeamsTab as DefaultOrganizerTeams,
} from "@/components/admin/organizer-dashboard-sections";
export { SubmissionsTable as DefaultOrganizerSubmissions } from "@/components/admin/submissions-table";
export { OrganizerAuditView as DefaultOrganizerAudit } from "@/components/admin/audit-log-view";
export type { OrganizerAuditViewProps } from "@/components/admin/audit-log-view";
export type { AdminAuditLog, AdminAuditLogPage } from "@/lib/admin-dashboard-types";
export { DefaultOrganizerOverviewView } from "@/components/admin/organizer-overview";
export type { OrganizerOverviewLink, OrganizerOverviewViewProps } from "@/components/admin/organizer-overview";
export { DefaultAnnouncementsView } from "@/components/admin/announcements-panel";
export { DefaultTrustedStateView } from "@/components/admin/trusted-state-panel";
export { DefaultParticipantManualView } from "@/components/docs/participant-manual";
export type { ManualStep, ParticipantManualViewProps } from "@/components/docs/participant-manual";
export { DefaultApiDocsView } from "@/components/docs/api-docs";
export type { ApiDocsViewProps } from "@/components/docs/api-docs";

// Logic hooks (already wired by the slots; use directly only for custom placements)
export { useFlagSubmit } from "@/components/dashboard/use-flag-submit";
export type {
  FlagSubmitNote,
  FlagSubmitState,
  FlagVerdict,
  FlagVerdictOutcome,
} from "@/components/dashboard/use-flag-submit";
export { useParticipantLogin } from "@/components/dashboard/use-participant-login";
export { useAnnouncements } from "@/components/admin/use-announcements";
export type { AnnouncementsState } from "@/components/admin/use-announcements";
export { toneForState, useTrustedState } from "@/components/admin/use-trusted-state";
export type { TrustedState, TrustTone } from "@/components/admin/use-trusted-state";
export type {
  ParticipantLoginMode,
  ParticipantLoginState,
} from "@/components/dashboard/use-participant-login";

// Display helpers
export {
  describeService,
  resolveServiceLockReason,
  SERVICE_UNLOCK_HINT,
  serviceCooldownLabel,
  serviceLockBadgeLabel,
  serviceLockHelpText,
} from "@/lib/service-view-model";
export type { ServiceLockReason, ServiceViewModel } from "@/lib/service-view-model";
export { cooldownMessage } from "@/lib/service-actions";
export type { ServiceActionFeedback, ServiceNotice } from "@/lib/service-actions";
export { formatSLAState, ServiceSLADetail } from "@/components/dashboard/service-sla-detail";
export {
  CHALLENGE_CATEGORIES,
  CHALLENGE_CATEGORY_LABELS,
  CHALLENGE_PROTOCOL_LABELS,
  challengeCategoryLabel,
  describeConnection,
} from "@/lib/challenge-metadata";
export type { ChallengeCategory, ChallengeProtocol, ServiceConnection } from "@/lib/challenge-metadata";
export { IssuedRootCredentialBlock } from "@/components/dashboard/ssh-credential-block";
export { buildAttackFilterOptions } from "@/lib/dashboard-utils";
export { cn } from "@/lib/utils";

// Data components
export { ScoreboardTable } from "@/components/ui/scoreboard-table";
export { RankBadge, ScoreboardRank } from "@/components/ui/scoreboard-rank";
export { AttackFeedTable } from "@/components/ui/attack-feed-table";
export { AttackMapPanel } from "@/components/ui/attack-map-panel";
export { AttackSliceSummaryGrid } from "@/components/ui/attack-slice-summary";
export { LiveModeBadge, PagedFilterActions, SliceCountBadge } from "@/components/ui/paged-filter-controls";

// UI primitives
export { AppearanceControls } from "@/components/ui/appearance-controls";
export { AppDialog } from "@/components/ui/app-dialog";
export { ChallengeCategoryIcon } from "@/components/ui/challenge-category-icon";
export { copyButtonLabel, ServiceConnectionLine, useCopyFeedback } from "@/components/ui/service-connection";
export type { CopyStatus } from "@/components/ui/service-connection";
export { Badge } from "@/components/ui/badge";
export { Button } from "@/components/ui/button";
export { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
export { EmptyStateText } from "@/components/ui/empty-state";
export { Input } from "@/components/ui/input";
export { Label } from "@/components/ui/label";
export { ScrollableRegion } from "@/components/ui/scrollable-region";
export { StatusBanner } from "@/components/ui/status-banner";
export { LinkPendingSignal } from "@/components/ui/link-pending";
export {
  ATTACKS_UNAVAILABLE,
  DataUnavailable,
  listUnavailableMessage,
  SCORES_UNAVAILABLE,
} from "@/components/ui/data-unavailable";
export { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
export { WorkspaceFrame } from "@/components/ui/workspace-frame";

// Data types
export type {
  AttackEvent,
  AttackFeedPage,
  PlatformOverview,
  ScoreRow,
  ServiceRow,
} from "@/lib/dashboard-types";
