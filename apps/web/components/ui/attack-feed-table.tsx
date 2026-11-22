"use client";

import { useRef, useState, type ReactElement } from "react";

import { Button } from "@/components/ui/button";
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
  first_blood?: boolean;
  id: string;
  tick: number;
  attacker: string;
  victim: string;
  service: string;
  verdict: string;
};

const ROW_BATCH = 50;

/**
 * Shared attack feed table used by both organizer and participant dashboards.
 * The attack map can load 1,000 rows, so the table adds them in batches.
 */
export function AttackFeedTable({
  attackRows,
  emptyMessage,
}: {
  attackRows: AttackRow[];
  emptyMessage?: string;
}): ReactElement {
  const [shownCount, setShownCount] = useState(ROW_BATCH);
  const countLabel = useRef<HTMLParagraphElement>(null);
  const rows = attackRows.slice(0, shownCount);
  const remaining = attackRows.length - rows.length;

  function showMore() {
    setShownCount(rows.length + ROW_BATCH);
    // The button is about to disappear, so keep keyboard focus nearby.
    if (remaining <= ROW_BATCH) requestAnimationFrame(() => countLabel.current?.focus());
  }

  return (
    <div className="grid min-w-0 gap-3">
    <Table scrollLabel="Accepted attacks">
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
          rows.map((attack) => (
            <TableRow key={attack.id}>
              <TableCell className="font-mono text-xs">
                #{attack.tick}
              </TableCell>
              <TableCell>{attack.attacker}</TableCell>
              <TableCell>{attack.victim}</TableCell>
              <TableCell>{attack.service}</TableCell>
              <TableCell>{attack.first_blood ? <span className="medal-gold mr-2 inline-block rounded-sm border px-1.5 py-0.5 text-xs font-semibold">First blood</span> : null}{attack.verdict}</TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
    {attackRows.length > ROW_BATCH ? (
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p ref={countLabel} tabIndex={-1} role="status" className="text-sm text-muted-foreground">
          Showing {rows.length.toLocaleString("en-US")} of {attackRows.length.toLocaleString("en-US")} attacks
        </p>
        {remaining > 0 ? (
          <Button type="button" variant="outline" size="sm" onClick={showMore}>
            Show {Math.min(remaining, ROW_BATCH)} more
          </Button>
        ) : null}
      </div>
    ) : null}
    </div>
  );
}
