import Link from "next/link";

import { OrganizerShell } from "@/components/admin/organizer-shell";
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
import { loadAdminDashboardData } from "@/lib/admin-dashboard-data";
import { listAdminAuditLogs } from "@/lib/admin-api";
import { formatIndonesianDate } from "@/lib/date-format";

type SearchParams = Record<string, string | string[] | undefined>;

type PageProps = {
  searchParams?: Promise<SearchParams>;
};

function firstValue(value: string | string[] | undefined) {
  if (Array.isArray(value)) {
    return value[0] ?? "";
  }
  return value ?? "";
}

function parsePositive(value: string, fallback: number) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  return parsed;
}

function buildPageHref(
  actorType: string,
  action: string,
  targetType: string,
  status: string,
  limit: number,
  offset: number,
) {
  const params = new URLSearchParams();
  if (actorType) {
    params.set("actor_type", actorType);
  }
  if (action) {
    params.set("action", action);
  }
  if (targetType) {
    params.set("target_type", targetType);
  }
  if (status) {
    params.set("status", status);
  }
  params.set("limit", String(limit));
  params.set("offset", String(Math.max(0, offset)));
  return `/admin/audit?${params.toString()}`;
}

function toneForStatus(status: string) {
  if (status === "success") {
    return "tone-success";
  }
  return "tone-neutral";
}

export default async function AdminAuditPage({ searchParams }: PageProps) {
  const params = (await searchParams) ?? {};
  const actorType = firstValue(params.actor_type).trim().toLowerCase();
  const action = firstValue(params.action).trim().toLowerCase();
  const targetType = firstValue(params.target_type).trim().toLowerCase();
  const status = firstValue(params.status).trim().toLowerCase();
  const limit = parsePositive(firstValue(params.limit), 25);
  const offset = Math.max(
    0,
    Number.parseInt(firstValue(params.offset), 10) || 0,
  );

  const [dashboard, auditPage] = await Promise.all([
    loadAdminDashboardData(),
    listAdminAuditLogs({
      actor_type: actorType || undefined,
      action: action || undefined,
      target_type: targetType || undefined,
      status: status || undefined,
      limit,
      offset,
    }),
  ]);

  return (
    <OrganizerShell
      activePath="/admin/audit"
      overview={dashboard.overview}
      title="Audit"
      description="Privileged participant and organizer actions recorded by the control plane."
    >
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
                  className="flex h-10 w-full rounded-md border border-input bg-background px-3 text-sm outline-none ring-offset-background transition focus-visible:ring-2 focus-visible:ring-ring"
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
                            className={toneForStatus(entry.status)}
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
                {auditPage.has_prev ? (
                  <Button asChild variant="outline">
                    <Link
                      href={buildPageHref(
                        actorType,
                        action,
                        targetType,
                        status,
                        limit,
                        Math.max(0, offset - limit),
                      )}
                    >
                      Previous
                    </Link>
                  </Button>
                ) : (
                  <Button variant="outline" disabled>
                    Previous
                  </Button>
                )}
                {auditPage.has_next ? (
                  <Button asChild variant="outline">
                    <Link
                      href={buildPageHref(
                        actorType,
                        action,
                        targetType,
                        status,
                        limit,
                        offset + limit,
                      )}
                    >
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
    </OrganizerShell>
  );
}
