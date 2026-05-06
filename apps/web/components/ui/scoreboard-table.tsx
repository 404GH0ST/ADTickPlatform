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

function MetricLine({
  icon,
  tone,
  value,
}: {
  icon: ReactElement;
  tone: string;
  value: number;
}): ReactElement {
  return (
    <div className={cn("flex items-center gap-1.5 font-mono text-[11px]", tone)}>
      <span className="shrink-0">{icon}</span>
      <span>{compactNumber(value)}</span>
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
      <div className="flex min-h-20 items-center justify-center rounded-md border border-border/50 bg-muted/25 text-[11px] text-muted-foreground">
        —
      </div>
    );
  }

  const tone =
    value.total > 0
      ? "border-emerald-500/30 bg-emerald-500/8"
      : "border-border/50 bg-muted/25";

  return (
    <div className={cn("min-h-20 rounded-md border px-2 py-2", tone)}>
      <div className="space-y-1.5">
        <MetricLine
          icon={<Flame className="h-3.5 w-3.5" />}
          tone="text-foreground"
          value={value.attack}
        />
        <MetricLine
          icon={<Shield className="h-3.5 w-3.5" />}
          tone="text-foreground"
          value={value.defense}
        />
        <MetricLine
          icon={<Gauge className="h-3.5 w-3.5" />}
          tone="text-muted-foreground"
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
        "h-auto min-w-28 bg-background px-3 py-3 text-xs font-semibold text-foreground",
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
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="sticky left-0 z-30 h-auto min-w-24 bg-background px-3 py-3 text-xs font-semibold text-foreground">
            Rank
          </TableHead>
          <TableHead className="sticky left-24 z-30 h-auto min-w-56 bg-background px-3 py-3 text-xs font-semibold text-foreground">
            Team
          </TableHead>
          {serviceColumns.map((service) => (
            <TableHead
              key={service.key}
              className="h-auto min-w-32 bg-background px-2 py-3 align-top text-xs font-semibold text-foreground"
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
          <TableHead className="sticky right-0 z-30 h-auto min-w-28 bg-background px-3 py-3 text-xs font-semibold text-foreground">
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

            return (
              <TableRow
                key={score.team}
                className={cn(
                  "border-b border-border/60 hover:bg-muted/20",
                  isCurrentTeam && "bg-primary/5",
                )}
              >
                <TableCell className="sticky left-0 z-20 bg-background/95 px-3 py-3 backdrop-blur supports-[backdrop-filter]:bg-background/80">
                  <ScoreboardRank
                    rank={score.rank}
                    delta={score.delta}
                    isCurrentTeam={isCurrentTeam}
                  />
                </TableCell>
                <TableCell
                  className={cn(
                    "sticky left-24 z-20 bg-background/95 px-3 py-3 font-semibold backdrop-blur supports-[backdrop-filter]:bg-background/80",
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
                <TableCell className="sticky right-0 z-20 bg-background/95 px-3 py-3 font-mono text-sm font-semibold backdrop-blur supports-[backdrop-filter]:bg-background/80">
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
