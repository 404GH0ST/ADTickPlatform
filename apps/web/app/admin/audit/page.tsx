import type { Metadata } from "next";
import { OrganizerShell } from "@/components/admin/organizer-shell";
import { ThemedAuditView } from "@/components/admin/themed-organizer-views";
import { loadAdminDashboardData } from "@/lib/admin-dashboard-data";
import { listAdminAuditLogs } from "@/lib/admin-api";

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

export const metadata: Metadata = { title: "Audit" };

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
    >
      <ThemedAuditView
        actorType={actorType}
        action={action}
        targetType={targetType}
        status={status}
        limit={limit}
        offset={offset}
        auditPage={auditPage}
        prevHref={
          auditPage.has_prev
            ? buildPageHref(actorType, action, targetType, status, limit, Math.max(0, offset - limit))
            : null
        }
        nextHref={
          auditPage.has_next
            ? buildPageHref(actorType, action, targetType, status, limit, offset + limit)
            : null
        }
      />
    </OrganizerShell>
  );
}
