import { NextResponse } from 'next/server';

import { listAdminGameSchedulerEvents } from '@/lib/admin-api';

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const query = {
    limit: parsePositiveInteger(searchParams.get('limit'), 12, 200),
    offset: parseNonNegativeInteger(searchParams.get('offset')),
    event_type: searchParams.get('event_type')?.trim().toLowerCase() || undefined,
    source: searchParams.get('source')?.trim().toLowerCase() || undefined,
    state: searchParams.get('state')?.trim().toLowerCase() || undefined,
  };

  try {
    const data = await listAdminGameSchedulerEvents(query);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler events failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}

function parsePositiveInteger(raw: string | null, fallback?: number, max = 0) {
  if (!raw) {
    return fallback;
  }
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback;
  }
  if (max > 0 && parsed > max) {
    return max;
  }
  return parsed;
}

function parseNonNegativeInteger(raw: string | null) {
  if (!raw) {
    return undefined;
  }
  const parsed = Number.parseInt(raw, 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return undefined;
  }
  return parsed;
}
