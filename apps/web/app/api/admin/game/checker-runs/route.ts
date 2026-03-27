import { NextRequest, NextResponse } from 'next/server';

import { listAdminCheckerRuns } from '@/lib/admin-api';

export async function GET(request: NextRequest) {
  const query = {
    limit: parsePositiveInteger(request.nextUrl.searchParams.get('limit'), 25, 200),
    offset: parseNonNegativeInteger(request.nextUrl.searchParams.get('offset')),
    tick_id: parsePositiveInteger(request.nextUrl.searchParams.get('tick_id'), undefined, 0),
    team_id: parsePositiveInteger(request.nextUrl.searchParams.get('team_id'), undefined, 0),
    challenge_id: parsePositiveInteger(request.nextUrl.searchParams.get('challenge_id'), undefined, 0),
    phase: request.nextUrl.searchParams.get('phase')?.trim().toLowerCase() || undefined,
    status: request.nextUrl.searchParams.get('status')?.trim().toLowerCase() || undefined,
  };

  try {
    const data = await listAdminCheckerRuns(query);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'checker runs failed';
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
