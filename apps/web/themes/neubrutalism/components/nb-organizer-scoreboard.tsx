"use client";

import { useState } from "react";
import { LoaderCircle, RefreshCw } from "lucide-react";

import {
  Button,
  Input,
  ScoreboardCategoryLeadersPanel,
  ScoreboardFreezeControl,
  ScoringAuditCard,
  type ThemeSlotProps,
} from "@/components/theme/kit";

import { NbStandings } from "./nb-standings";

/**
 * Organizer scoreboard: the same standings design participants see (plus rank
 * movement), then freeze, replay audit and award leaders from the stock tab.
 */
export function NbOrganizerScoreboard({
  pendingAction,
  scoringAudit,
  scoreRows,
  onAudit,
  onRefresh,
}: ThemeSlotProps["OrganizerScoreboard"]) {
  const [teamFilter, setTeamFilter] = useState("");
  const query = teamFilter.trim().toLowerCase();
  const rows = query === "" ? scoreRows : scoreRows.filter((row) => row.team.toLowerCase().includes(query));

  return (
    <div className="grid grid-cols-1 gap-6">
      <NbStandings
        testId="authoritative-scoreboard-card"
        title="Live standings"
        rows={rows}
        teamCount={scoreRows.length}
        showDelta
        emptyMessage={query ? "No teams match this filter." : "No score rows persisted yet."}
        meta={
          <span role="status" aria-live="polite" className="text-xs font-semibold text-muted-foreground tabular-nums">
            Showing {rows.length} of {scoreRows.length} teams.
          </span>
        }
        toolbar={
          <>
            <label htmlFor="scoreboard-team-filter" className="nb-eyebrow text-[0.6875rem] text-muted-foreground">
              Team filter
            </label>
            <Input
              id="scoreboard-team-filter"
              className="h-9 w-44 max-w-full sm:w-56"
              placeholder="Filter teams"
              value={teamFilter}
              onChange={(event) => setTeamFilter(event.target.value)}
            />
            <Button disabled={pendingAction !== null} size="sm" variant="outline" onClick={onRefresh}>
              {pendingAction === "game:scoreboard" ? (
                <LoaderCircle className="size-4 animate-spin" aria-hidden />
              ) : (
                <RefreshCw className="size-4" aria-hidden />
              )}
              Refresh
            </Button>
          </>
        }
      />
      <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-2">
        <ScoreboardFreezeControl />
        <ScoringAuditCard pendingAction={pendingAction} scoringAudit={scoringAudit} onAudit={onAudit} />
      </div>
      <ScoreboardCategoryLeadersPanel scoreRows={scoreRows} />
    </div>
  );
}
