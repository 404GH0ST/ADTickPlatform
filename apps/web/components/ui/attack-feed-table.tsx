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

type AttackRow = {
  id: string;
  tick: number;
  attacker: string;
  victim: string;
  service: string;
  verdict: string;
};

/**
 * Shared attack feed table used by both organizer and participant dashboards.
 */
export function AttackFeedTable({
  attackRows,
  emptyMessage,
}: {
  attackRows: AttackRow[];
  emptyMessage?: string;
}): ReactElement {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Tick</TableHead>
          <TableHead>Attacker</TableHead>
          <TableHead>Victim</TableHead>
          <TableHead>Service</TableHead>
          <TableHead>Verdict</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {attackRows.length === 0 && emptyMessage ? (
          <EmptyTableRow colSpan={5} message={emptyMessage} />
        ) : (
          attackRows.map((attack) => (
            <TableRow key={attack.id}>
              <TableCell className="font-mono text-xs">
                #{attack.tick}
              </TableCell>
              <TableCell>{attack.attacker}</TableCell>
              <TableCell>{attack.victim}</TableCell>
              <TableCell>{attack.service}</TableCell>
              <TableCell>{attack.verdict}</TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}
