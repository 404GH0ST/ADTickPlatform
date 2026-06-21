"use client";

import { useState, useMemo, type ReactElement } from "react";
import { Flame, Gauge, Shield, ArrowUp, ArrowDown } from "lucide-react";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { EmptyTableRow } from "@/components/ui/empty-state";
import { ScoreboardRank, RankBadge } from "@/components/ui/scoreboard-rank";
import { cn } from "@/lib/utils";

export type ScoreboardServiceRow = {
  challenge_id: number;
  service: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
};

export type ScoreboardRow = {
  rank: number;
  delta: string;
  team: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
  services?: ScoreboardServiceRow[];
};

type ServiceColumn = {
  key: string;
  challengeID: number;
  service: string;
};

function serviceColumnKey(service: ScoreboardServiceRow): string {
  return `${service.challenge_id}:${service.service}`;
}

function collectServiceColumns(scoreRows: ScoreboardRow[]): ServiceColumn[] {
  const seen = new Set<string>();
  const columns: ServiceColumn[] = [];
  for (const row of scoreRows) {
    for (const service of row.services ?? []) {
      const key = serviceColumnKey(service);
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      columns.push({
        key,
        challengeID: service.challenge_id,
        service: service.service,
      });
    }
  }
  return columns.sort((a, b) => {
    if (a.challengeID !== b.challengeID) {
      return a.challengeID - b.challengeID;
    }
    return a.service.localeCompare(b.service);
  });
}

function serviceLookup(row: ScoreboardRow): Map<string, ScoreboardServiceRow> {
  return new Map(
    (row.services ?? []).map((service) => [serviceColumnKey(service), service]),
  );
}

function compactNumber(value: number): string {
  return Intl.NumberFormat("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

function scoreTone(value: number, fallback = "text-foreground"): string {
  if (value > 0) {
    return "text-positive";
  }
  if (value < 0) {
    return "text-negative";
  }
  return fallback;
}

function MetricLine({
  icon,
  label,
  shortLabel,
  tone,
  value,
}: {
  icon: ReactElement;
  label: string;
  shortLabel: string;
  tone: string;
  value: number;
}): ReactElement {
  return (
    <div
      aria-label={`${label}: ${compactNumber(value)}`}
      className={cn("flex min-w-0 items-center gap-1.5 font-mono text-[11px]", tone)}
    >
      <span className="shrink-0">{icon}</span>
      <span className="w-6 shrink-0 text-[10px] font-semibold uppercase tracking-normal text-muted-foreground">
        {shortLabel}
      </span>
      <span className="truncate">{compactNumber(value)}</span>
    </div>
  );
}

function ServiceCell({
  value,
}: {
  value?: ScoreboardServiceRow;
}): ReactElement {
  if (!value) {
    return (
      <div className="flex h-16 items-center justify-center rounded-sm border border-border/20 bg-muted/10 text-muted-foreground/35 select-none">
        —
      </div>
    );
  }

  const totalTone = scoreTone(value.total);
  const isPositive = value.total > 0;
  const isNegative = value.total < 0;

  return (
    <div
      className={cn(
        "flex flex-col gap-1 rounded-sm border p-2 transition-colors",
        isPositive && "bg-positive/5 border-positive/30 hover:bg-positive/10",
        isNegative && "bg-negative/5 border-negative/30 hover:bg-negative/10",
        !isPositive && !isNegative && "bg-muted/15 border-border/30 hover:bg-muted/25"
      )}
      aria-label={`Service ${value.service}: total ${compactNumber(value.total)}, attack ${compactNumber(value.attack)}, defense ${compactNumber(value.defense)}, SLA ${compactNumber(value.sla)}%`}
    >
      <MetricLine
        icon={<Flame className="h-3.5 w-3.5" />}
        label="Attack"
        shortLabel="A"
        tone={scoreTone(value.attack)}
        value={value.attack}
      />
      <MetricLine
        icon={<Shield className="h-3.5 w-3.5" />}
        label="Defense"
        shortLabel="D"
        tone={scoreTone(value.defense)}
        value={value.defense}
      />
      <MetricLine
        icon={<Gauge className="h-3.5 w-3.5" />}
        label="SLA"
        shortLabel="SLA"
        tone={scoreTone(value.sla, "text-muted-foreground")}
        value={value.sla}
      />
      <div className="border-t border-border/60 pt-1 flex justify-between text-[11px] font-mono font-bold">
        <span className="text-muted-foreground">Total:</span>
        <span className={totalTone}>
          {value.total > 0 ? "+" : ""}
          {compactNumber(value.total)}
        </span>
      </div>
    </div>
  );
}



/**
 * Shared scoreboard table used by both organizer and participant dashboards.
 */
export function ScoreboardTable({
  scoreRows,
  emptyMessage,
  currentTeamName,
}: {
  scoreRows: ScoreboardRow[];
  emptyMessage: string;
  currentTeamName?: string;
}): ReactElement {
  const [sortField, setSortField] = useState<
    "rank" | "team" | "attack" | "defense" | "sla" | "total"
  >("rank");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");

  const sortedRows = useMemo(() => {
    return [...scoreRows].sort((left, right) => {
      const direction = sortDirection === "asc" ? 1 : -1;
      switch (sortField) {
        case "team":
          return left.team.localeCompare(right.team) * direction;
        case "attack":
          return (left.attack - right.attack) * direction;
        case "defense":
          return (left.defense - right.defense) * direction;
        case "sla":
          return (left.sla - right.sla) * direction;
        case "total":
          return (left.total - right.total) * direction;
        case "rank":
        default:
          return (left.rank - right.rank) * direction;
      }
    });
  }, [scoreRows, sortField, sortDirection]);

  const handleSort = (field: typeof sortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === "asc" ? "desc" : "asc");
    } else {
      setSortField(field);
      setSortDirection("asc");
    }
  };

  const renderSortHeader = (
    field: typeof sortField,
    label: string,
    icon?: ReactElement,
  ) => {
    const isActive = sortField === field;
    return (
      <button
        type="button"
        onClick={() => handleSort(field)}
        className={cn(
          "flex items-center gap-1.5 hover:text-foreground text-left focus:outline-none focus:ring-1 focus:ring-ring rounded-sm px-1.5 -mx-1.5 py-1 transition-colors w-full font-semibold group/sort",
          isActive ? "text-foreground" : "text-muted-foreground",
        )}
      >
        {icon}
        <span className="truncate">{label}</span>
        <span className="shrink-0 ml-auto">
          {isActive ? (
            sortDirection === "asc" ? (
              <ArrowUp className="h-3.5 w-3.5" />
            ) : (
              <ArrowDown className="h-3.5 w-3.5" />
            )
          ) : (
            <ArrowUp className="h-3.5 w-3.5 text-muted-foreground/30 opacity-0 group-hover/sort:opacity-100 transition-opacity" />
          )}
        </span>
      </button>
    );
  };

  const serviceColumns = collectServiceColumns(scoreRows);
  const columnCount = 6 + serviceColumns.length;
  const desktopTableWidthRem = 7 + 14 + serviceColumns.length * 8 + 28;

  return (
    <>
      <MobileScoreboardCards
        scoreRows={sortedRows}
        emptyMessage={emptyMessage}
        currentTeamName={currentTeamName}
        sortField={sortField}
        sortDirection={sortDirection}
        onSortFieldChange={setSortField}
        onSortDirectionChange={setSortDirection}
      />
      <Table
        className="hidden border-separate border-spacing-0 text-xs md:table"
        style={{
          width: `${desktopTableWidthRem}rem`,
          minWidth: `${desktopTableWidthRem}rem`,
          tableLayout: "fixed",
        }}
      >
      <caption className="caption-bottom px-2 py-3 text-left">
        <span className="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
          <span><span className="font-semibold text-foreground">A</span> Attack</span>
          <span><span className="font-semibold text-foreground">D</span> Defense</span>
          <span><span className="font-semibold text-foreground">SLA</span> Availability</span>
          <span><span className="font-semibold text-positive">Green +</span></span>
          <span><span className="font-semibold text-negative">Red -</span></span>
          <span>Neutral cell background</span>
        </span>
      </caption>
      <colgroup>
        <col className="w-28" />
        <col className="w-56" />
        {serviceColumns.map((service) => (
          <col key={service.key} className="w-32" />
        ))}
        <col className="w-28" />
        <col className="w-28" />
        <col className="w-28" />
        <col className="w-28" />
      </colgroup>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="sticky left-0 z-30 h-auto w-28 min-w-28 max-w-28 border-r border-border/70 bg-card px-3 py-2 text-xs font-semibold text-foreground" scope="col">
            {renderSortHeader("rank", "Rank")}
          </TableHead>
          <TableHead className="sticky left-28 z-30 h-auto w-56 min-w-56 max-w-56 border-r border-border/70 bg-card px-3 py-2 text-xs font-semibold text-foreground" scope="col">
            {renderSortHeader("team", "Team")}
          </TableHead>
          {serviceColumns.map((service) => (
            <TableHead
              key={service.key}
              className="h-auto w-32 min-w-32 max-w-32 bg-card px-2 py-3 align-top text-xs font-semibold text-foreground"
              scope="col"
            >
              <div className="space-y-1">
                <div className="truncate">{service.service}</div>
                <div className="font-mono text-[10px] text-muted-foreground">
                  A / D / SLA
                </div>
              </div>
            </TableHead>
          ))}
          <TableHead className="h-auto w-28 min-w-28 max-w-28 bg-card px-3 py-2 text-xs font-semibold text-foreground" scope="col">
            {renderSortHeader("attack", "Total Offense", <Flame className="h-3.5 w-3.5" />)}
          </TableHead>
          <TableHead className="h-auto w-28 min-w-28 max-w-28 bg-card px-3 py-2 text-xs font-semibold text-foreground" scope="col">
            {renderSortHeader("defense", "Total Defense", <Shield className="h-3.5 w-3.5" />)}
          </TableHead>
          <TableHead className="h-auto w-28 min-w-28 max-w-28 bg-card px-3 py-2 text-xs font-semibold text-foreground" scope="col">
            {renderSortHeader("sla", "Total SLA", <Gauge className="h-3.5 w-3.5" />)}
          </TableHead>
          <TableHead className="sticky right-0 z-30 h-auto w-28 min-w-28 max-w-28 border-l border-border/70 bg-card px-3 py-2 text-xs font-semibold text-foreground" scope="col">
            {renderSortHeader("total", "Total")}
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sortedRows.length === 0 ? (
          <EmptyTableRow colSpan={columnCount} message={emptyMessage} />
        ) : (
          sortedRows.map((score) => {
            const isCurrentTeam =
              currentTeamName !== undefined && score.team === currentTeamName;
            const services = serviceLookup(score);
            const stickyCellClassName = isCurrentTeam
              ? "bg-[color-mix(in_oklab,var(--card)_88%,var(--primary))]"
              : "bg-card";

            return (
              <TableRow
                key={score.team}
                className={cn(
                  "border-b border-border/60 transition-colors hover:bg-muted/20",
                  isCurrentTeam && "bg-primary/5",
                )}
              >
                <TableCell
                  className={cn(
                    "sticky left-0 z-20 w-28 min-w-28 max-w-28 border-r border-border/70 px-3 py-3",
                    stickyCellClassName,
                  )}
                >
                  <ScoreboardRank
                    rank={score.rank}
                    delta={score.delta}
                    isCurrentTeam={isCurrentTeam}
                  />
                </TableCell>
                <TableCell
                  className={cn(
                    "sticky left-28 z-20 w-56 min-w-56 max-w-56 overflow-hidden border-r border-border/70 px-3 py-3 font-semibold",
                    stickyCellClassName,
                    isCurrentTeam && "text-primary",
                  )}
                >
                  <div className="block truncate text-sm" title={score.team}>
                    {score.team}
                  </div>
                </TableCell>
                {serviceColumns.map((service) => (
                  <TableCell key={service.key} className="w-32 min-w-32 max-w-32 px-2 py-3">
                    <ServiceCell value={services.get(service.key)} />
                  </TableCell>
                ))}
                <TableCell className="w-28 min-w-28 max-w-28 px-3 py-3 font-mono text-sm">
                  {compactNumber(score.attack)}
                </TableCell>
                <TableCell className="w-28 min-w-28 max-w-28 px-3 py-3 font-mono text-sm">
                  {compactNumber(score.defense)}
                </TableCell>
                <TableCell className="w-28 min-w-28 max-w-28 px-3 py-3 font-mono text-sm">
                  {compactNumber(score.sla)}
                </TableCell>
                <TableCell
                  className={cn(
                    "sticky right-0 z-20 w-28 min-w-28 max-w-28 border-l border-border/70 px-3 py-3 font-mono text-sm font-semibold",
                    stickyCellClassName,
                  )}
                >
                  {compactNumber(score.total)}
                </TableCell>
              </TableRow>
            );
          })
        )}
      </TableBody>
      </Table>
    </>
  );
}

function MobileScoreboardCards({
  currentTeamName,
  emptyMessage,
  scoreRows,
  sortField,
  sortDirection,
  onSortFieldChange,
  onSortDirectionChange,
}: {
  currentTeamName?: string;
  emptyMessage: string;
  scoreRows: ScoreboardRow[];
  sortField: "rank" | "team" | "attack" | "defense" | "sla" | "total";
  sortDirection: "asc" | "desc";
  onSortFieldChange: (field: any) => void;
  onSortDirectionChange: (dir: any) => void;
}): ReactElement {
  const [expandedTeams, setExpandedTeams] = useState<Record<string, boolean>>({});

  if (scoreRows.length === 0) {
    return (
      <div className="rounded-sm border border-border/55 bg-muted/20 p-4 text-sm text-muted-foreground md:hidden">
        {emptyMessage}
      </div>
    );
  }

  const toggleExpand = (teamName: string) => {
    setExpandedTeams((prev) => ({
      ...prev,
      [teamName]: !prev[teamName],
    }));
  };

  return (
    <div className="grid gap-3 md:hidden">
      <div className="flex flex-col gap-2 rounded-sm border border-border/75 bg-muted/10 p-2.5">
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-muted-foreground">
          <span><span className="font-semibold text-foreground">A</span> Attack</span>
          <span><span className="font-semibold text-foreground">D</span> Defense</span>
          <span><span className="font-semibold text-foreground">SLA</span> Availability</span>
          <span><span className="font-semibold text-positive">Green +</span></span>
          <span><span className="font-semibold text-negative">Red -</span></span>
        </div>
        <div className="flex items-center gap-2 border-t border-border/50 pt-2 text-xs">
          <label htmlFor="mobile-sort" className="font-semibold text-muted-foreground">
            Sort by:
          </label>
          <select
            id="mobile-sort"
            className="h-8 flex-1 rounded-sm border border-border/70 bg-card px-2 outline-none text-[11px]"
            value={sortField}
            onChange={(e) => onSortFieldChange(e.target.value)}
          >
            <option value="rank">Rank</option>
            <option value="team">Team Name</option>
            <option value="attack">Total Offense</option>
            <option value="defense">Total Defense</option>
            <option value="sla">Total SLA</option>
            <option value="total">Total Score</option>
          </select>
          <select
            id="mobile-sort-dir"
            className="h-8 rounded-sm border border-border/70 bg-card px-2 outline-none text-[11px]"
            value={sortDirection}
            onChange={(e) => onSortDirectionChange(e.target.value)}
          >
            <option value="asc">Ascending</option>
            <option value="desc">Descending</option>
          </select>
        </div>
      </div>
      {scoreRows.map((score) => {
        const isCurrentTeam =
          currentTeamName !== undefined && score.team === currentTeamName;
        const isExpanded = !!expandedTeams[score.team];
        const allServices = score.services ?? [];
        const visibleServices = isExpanded ? allServices : allServices.slice(0, 3);

        return (
          <article
            key={score.team}
            className={cn(
              "rounded-sm border border-border/65 bg-card p-3",
              isCurrentTeam && "border-primary/70 bg-primary/5",
            )}
            aria-label={`Rank ${score.rank}, ${score.team}, total ${compactNumber(score.total)}`}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="mb-1.5 flex items-center">
                  <RankBadge rank={score.rank} />
                </div>
                <p className={cn("truncate text-base font-semibold", isCurrentTeam && "text-primary")}>
                  {score.team}
                </p>
              </div>
              <div className="text-right font-mono">
                <p className="text-[10px] font-semibold uppercase text-muted-foreground">
                  Total
                </p>
                <p className="text-base font-semibold text-foreground">
                  {compactNumber(score.total)}
                </p>
              </div>
            </div>
            <div className="mt-3 grid grid-cols-3 gap-2 text-xs">
              <ScoreSummary label="A" tone={scoreTone(score.attack)} value={score.attack} />
              <ScoreSummary label="D" tone={scoreTone(score.defense)} value={score.defense} />
              <ScoreSummary label="SLA" tone={scoreTone(score.sla, "text-muted-foreground")} value={score.sla} />
            </div>
            {allServices.length > 0 ? (
              <div className="mt-3 space-y-2">
                {visibleServices.map((service) => (
                  <div
                    key={serviceColumnKey(service)}
                    className="rounded-sm border border-border/55 bg-muted/20 p-2"
                  >
                    <p className="truncate text-xs font-semibold text-foreground">
                      {service.service}
                    </p>
                    <div className="mt-2 grid grid-cols-3 gap-2 text-[11px]">
                      <ScoreSummary label="A" tone={scoreTone(service.attack)} value={service.attack} />
                      <ScoreSummary label="D" tone={scoreTone(service.defense)} value={service.defense} />
                      <ScoreSummary label="SLA" tone={scoreTone(service.sla, "text-muted-foreground")} value={service.sla} />
                    </div>
                  </div>
                ))}
                {allServices.length > 3 ? (
                  <button
                    type="button"
                    onClick={() => toggleExpand(score.team)}
                    className="w-full mt-2 text-center py-1.5 border border-dashed border-border/70 hover:bg-muted/15 text-xs font-semibold rounded-sm text-muted-foreground hover:text-foreground transition-all cursor-pointer"
                  >
                    {isExpanded ? "Collapse services view" : `Show all services (+${allServices.length - 3} more)`}
                  </button>
                ) : null}
              </div>
            ) : null}
          </article>
        );
      })}
    </div>
  );
}

function ScoreSummary({
  label,
  tone,
  value,
}: {
  label: string;
  tone: string;
  value: number;
}): ReactElement {
  return (
    <div className="min-w-0 font-mono">
      <p className="text-[10px] font-semibold uppercase text-muted-foreground">
        {label}
      </p>
      <p className={cn("truncate text-xs font-semibold", tone)}>
        {compactNumber(value)}
      </p>
    </div>
  );
}
