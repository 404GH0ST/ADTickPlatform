"use client";

import {
  ChallengesTab,
  DeploymentsTab,
  GameAttacksCard,
  GameTab,
  PlayersTab,
  ScoreboardTab,
  TeamsTab,
} from "@/components/admin/organizer-dashboard-sections";
import { SubmissionsTable } from "@/components/admin/submissions-table";
import { OrganizerAuditView } from "@/components/admin/audit-log-view";
import { themed } from "@/components/theme/theme-slot";

/*
 * Organizer views routed through theme slots. Each takes exactly the stock
 * component's props; the active theme may replace it (see themes/README.md).
 */
export const ThemedTeamsTab = themed("OrganizerTeams", TeamsTab);
export const ThemedPlayersTab = themed("OrganizerPlayers", PlayersTab);
export const ThemedChallengesTab = themed("OrganizerChallenges", ChallengesTab);
export const ThemedDeploymentsTab = themed("OrganizerDeployments", DeploymentsTab);
export const ThemedGameTab = themed("OrganizerGame", GameTab);
export const ThemedScoreboardTab = themed("OrganizerScoreboard", ScoreboardTab);
export const ThemedAttacksCard = themed("OrganizerAttacks", GameAttacksCard);
export const ThemedSubmissionsTable = themed("OrganizerSubmissions", SubmissionsTable);
export const ThemedAuditView = themed("OrganizerAudit", OrganizerAuditView);
