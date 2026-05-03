import { NextRequest, NextResponse } from 'next/server';

import { listAdminCheckerRuns } from '@/lib/admin-api';
import {
  parseLowercaseTextFilter,
  parseNonNegativeInteger,
  parsePositiveInteger,
} from '@/lib/route-query';

export async function GET(request: NextRequest) {
  const query = {
    limit: parsePositiveInteger(request.nextUrl.searchParams.get('limit'), 25, 200),
    offset: parseNonNegativeInteger(request.nextUrl.searchParams.get('offset')),
    tick_id: parsePositiveInteger(request.nextUrl.searchParams.get('tick_id'), undefined, 0),
    team_id: parsePositiveInteger(request.nextUrl.searchParams.get('team_id'), undefined, 0),
    challenge_id: parsePositiveInteger(request.nextUrl.searchParams.get('challenge_id'), undefined, 0),
    phase: parseLowercaseTextFilter(request.nextUrl.searchParams.get('phase')),
    status: parseLowercaseTextFilter(request.nextUrl.searchParams.get('status')),
  };

  try {
    const data = await listAdminCheckerRuns(query);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'checker runs failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
