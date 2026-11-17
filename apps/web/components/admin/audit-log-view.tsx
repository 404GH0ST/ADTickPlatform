"use client";

import Link from "next/link";
import type { ReactElement } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { EmptyTableRow } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { AdminAuditLogPage } from "@/lib/admin-dashboard-types";
import { formatIndonesianDate } from "@/lib/date-format";

/**
 * Data for the organizer audit page. Filters are applied by a plain GET form
 * (field names actor_type, action, target_type, status, limit, offset), so a
 * themed view only has to render the form and links; paging is server-side.
 */
export type OrganizerAuditViewProps = {
  actorType: string;
  action: string;
  targetType: string;
  status: string;
  limit: number;
  offset: number;
  auditPage: AdminAuditLogPage;
  /** Link to the previous page, or null on the first page. */
  prevHref: string | null;
  /** Link to the next page, or null on the last page. */
  nextHref: string | null;
};

export function auditStatusTone(status: string) {
  const norm = status.trim().toLowerCase();
  if (norm === "success") {
    return "tone-success";
  }
  if (norm === "fail" || norm === "failed" || norm === "error") {
    return "tone-danger";
  }
  return "tone-neutral";
}

export function OrganizerAuditView({
  actorType,
  action,
  targetType,
  status,
  limit,
  offset,
  auditPage,
  prevHref,
  nextHref,
}: OrganizerAuditViewProps): ReactElement {
  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Filters</CardTitle>
          <CardDescription>
            Filter by actor, action, target type, or status. Paging is
            server-side.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form className="grid gap-3 md:grid-cols-5">
            <div className="space-y-2">
              <Label htmlFor="actor_type">Actor Type</Label>
              <Input
                id="actor_type"
                name="actor_type"
                defaultValue={actorType}
                placeholder="team or admin"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="action">Action</Label>
              <Input
                id="action"
                name="action"
                defaultValue={action}
                placeholder="service.unlock"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="target_type">Target Type</Label>
              <Input
                id="target_type"
                name="target_type"
                defaultValue={targetType}
                placeholder="service"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="status">Status</Label>
              <Input
                id="status"
                name="status"
                defaultValue={status}
                placeholder="success"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="limit">Page Size</Label>
              <select
                id="limit"
                name="limit"
                defaultValue={String(limit)}
                className="flex h-10 w-full rounded-sm border border-input bg-background px-3 text-sm outline-none ring-offset-background transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="25">25 per page</option>
                <option value="50">50 per page</option>
                <option value="100">100 per page</option>
              </select>
            </div>
            <input type="hidden" name="offset" value="0" />
            <div className="flex items-end gap-2 md:col-span-5">
              <Button type="submit" variant="outline">
                Apply Filters
              </Button>
              <Button asChild variant="outline">
                <Link href="/admin/audit">Reset</Link>
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Audit Log</CardTitle>
          <CardDescription>
            Showing {auditPage.items.length > 0 ? offset + 1 : 0}-
            {offset + auditPage.items.length} of {auditPage.total_count} audit
            entries.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>When</TableHead>
                  <TableHead>Actor</TableHead>
                  <TableHead>Action</TableHead>
                  <TableHead>Target</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Message</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {auditPage.items.length === 0 ? (
                  <EmptyTableRow
                    colSpan={6}
                    message="No audit entries matched the current filter."
                  />
                ) : (
                  auditPage.items.map((entry) => (
                    <TableRow key={entry.id}>
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                        {formatIndonesianDate(entry.created_at)}
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        <div className="font-medium">{entry.actor}</div>
                        <div className="text-xs text-muted-foreground">
                          {entry.actor_type}
                        </div>
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        {entry.action}
                      </TableCell>
                      <TableCell>
                        <div>{entry.target}</div>
                        <div className="text-xs text-muted-foreground">
                          {entry.target_type}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant="outline"
                          className={auditStatusTone(entry.status)}
                        >
                          {entry.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="min-w-64">
                        <div>{entry.message}</div>
                        {entry.metadata !== "{}" ? (
                          <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-all text-[11px] text-muted-foreground">
                            {entry.metadata}
                          </pre>
                        ) : null}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          <div className="flex flex-wrap items-center justify-end gap-2 border-t pt-3">
            <div className="flex gap-2">
              {prevHref ? (
                <Button asChild variant="outline">
                  <Link href={prevHref}>
                    Previous
                  </Link>
                </Button>
              ) : (
                <Button variant="outline" disabled>
                  Previous
                </Button>
              )}
              {nextHref ? (
                <Button asChild variant="outline">
                  <Link href={nextHref}>
                    Next
                  </Link>
                </Button>
              ) : (
                <Button variant="outline" disabled>
                  Next
                </Button>
              )}
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
