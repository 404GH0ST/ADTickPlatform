import type { ReactElement } from "react";

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

export type ScoreboardRow = {
  rank: number;
  delta: string;
  team: string;
  attack: number;
  defense: number;
  sla: number;
  total: number;
};

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
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-[180px]">Rank</TableHead>
          <TableHead>Team</TableHead>
          <TableHead>Attack</TableHead>
          <TableHead>Defense</TableHead>
          <TableHead>SLA</TableHead>
          <TableHead>Total</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {scoreRows.length === 0 ? (
          <EmptyTableRow
            colSpan={6}
            message={emptyMessage}
          />
        ) : (
          scoreRows.map((score) => (
            <TableRow key={score.team}>
              <TableCell>
                <ScoreboardRank
                  rank={score.rank}
                  delta={score.delta}
                  isCurrentTeam={currentTeamName ? score.team === currentTeamName : undefined}
                />
              </TableCell>
              <TableCell
                className={cn(
                  "font-semibold",
                  currentTeamName && score.team === currentTeamName && "text-primary",
                )}
              >
                {score.team}
              </TableCell>
              <TableCell>{score.attack}</TableCell>
              <TableCell>{score.defense}</TableCell>
              <TableCell>{score.sla}</TableCell>
              <TableCell className="font-semibold">{score.total}</TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}
