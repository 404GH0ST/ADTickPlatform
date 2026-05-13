import type { ReactElement } from "react";
import { Flame, Gauge, Shield } from "lucide-react";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { EmptyTableRow } from "@/components/ui/empty-state";
import { ScoreboardRank } from "@/components/ui/scoreboard-rank";
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
  tone,
  value,
}: {
  icon: ReactElement;
  label: string;
  tone: string;
  value: number;
}): ReactElement {
  return (
    <div
      aria-label={`${label}: ${compactNumber(value)}`}
      className={cn("flex min-w-0 items-center gap-1.5 font-mono text-[11px]", tone)}
    >
      <span className="shrink-0">{icon}</span>
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
      <div className="flex min-h-16 items-center justify-center rounded-sm border border-border/55 bg-muted/20 text-[11px] text-muted-foreground">
        —
      </div>
    );
  }

  return (
    <div
      aria-label={`Attack ${compactNumber(value.attack)}, defense ${compactNumber(value.defense)}, SLA ${compactNumber(value.sla)}, total ${compactNumber(value.total)}`}
      className="min-h-16 rounded-sm border border-border/55 bg-muted/20 px-2 py-2"
      role="group"
    >
      <div className="grid gap-1.5">
        <MetricLine
          icon={<Flame className="h-3.5 w-3.5" />}
          label="Attack"
          tone={scoreTone(value.attack)}
          value={value.attack}
        />
        <MetricLine
          icon={<Shield className="h-3.5 w-3.5" />}
          label="Defense"
          tone={scoreTone(value.defense)}
          value={value.defense}
        />
        <MetricLine
          icon={<Gauge className="h-3.5 w-3.5" />}
          label="SLA"
          tone={scoreTone(value.sla, "text-muted-foreground")}
          value={value.sla}
        />
      </div>
    </div>
  );
}

function SummaryHeader({
  icon,
  label,
  sticky,
}: {
  icon: ReactElement;
  label: string;
  sticky?: string;
}): ReactElement {
  return (
    <TableHead
      className={cn(
        "h-auto min-w-28 bg-card px-3 py-3 text-xs font-semibold text-foreground",
        sticky,
      )}
    >
      <div className="flex items-center gap-1.5 whitespace-nowrap">
        {icon}
        <span>{label}</span>
      </div>
    </TableHead>
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
  const serviceColumns = collectServiceColumns(scoreRows);
  const columnCount = 6 + serviceColumns.length;

  return (
    <Table className="min-w-[980px] border-separate border-spacing-0 text-xs">
      <caption className="sr-only">
        Scoreboard rankings by team. Service cells list attack, defense, and SLA scores. Green text means a positive score; red text means a negative score. Cell background is not checker health.
      </caption>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="sticky left-0 z-30 h-auto min-w-24 border-r border-border/70 bg-card px-3 py-3 text-xs font-semibold text-foreground" scope="col">
            Rank
          </TableHead>
          <TableHead className="sticky left-24 z-30 h-auto min-w-56 border-r border-border/70 bg-card px-3 py-3 text-xs font-semibold text-foreground" scope="col">
            Team
          </TableHead>
          {serviceColumns.map((service) => (
            <TableHead
              key={service.key}
              className="h-auto min-w-32 bg-card px-2 py-3 align-top text-xs font-semibold text-foreground"
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
          <SummaryHeader
            icon={<Flame className="h-3.5 w-3.5" />}
            label="Total Offense"
          />
          <SummaryHeader
            icon={<Shield className="h-3.5 w-3.5" />}
            label="Total Defense"
          />
          <SummaryHeader
            icon={<Gauge className="h-3.5 w-3.5" />}
            label="Total SLA"
          />
          <TableHead className="sticky right-0 z-30 h-auto min-w-28 border-l border-border/70 bg-card px-3 py-3 text-xs font-semibold text-foreground" scope="col">
            Total
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {scoreRows.length === 0 ? (
          <EmptyTableRow colSpan={columnCount} message={emptyMessage} />
        ) : (
          scoreRows.map((score) => {
            const isCurrentTeam =
              currentTeamName !== undefined && score.team === currentTeamName;
            const services = serviceLookup(score);
            const stickyCellClassName = isCurrentTeam ? "bg-primary/10" : "bg-card";

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
                    "sticky left-0 z-20 border-r border-border/70 px-3 py-3",
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
                    "sticky left-24 z-20 border-r border-border/70 px-3 py-3 font-semibold",
                    stickyCellClassName,
                    isCurrentTeam && "text-primary",
                  )}
                >
                  <div className="min-w-40 text-sm">{score.team}</div>
                </TableCell>
                {serviceColumns.map((service) => (
                  <TableCell key={service.key} className="px-2 py-3">
                    <ServiceCell value={services.get(service.key)} />
                  </TableCell>
                ))}
                <TableCell className="px-3 py-3 font-mono text-sm">
                  {compactNumber(score.attack)}
                </TableCell>
                <TableCell className="px-3 py-3 font-mono text-sm">
                  {compactNumber(score.defense)}
                </TableCell>
                <TableCell className="px-3 py-3 font-mono text-sm">
                  {compactNumber(score.sla)}
                </TableCell>
                <TableCell
                  className={cn(
                    "sticky right-0 z-20 border-l border-border/70 px-3 py-3 font-mono text-sm font-semibold",
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
  );
}
