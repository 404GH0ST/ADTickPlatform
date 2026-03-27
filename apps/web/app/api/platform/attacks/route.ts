import { NextRequest, NextResponse } from 'next/server';

import { listAttackFeed } from '@/lib/platform-api';

export async function GET(request: NextRequest) {
  const query = {
    limit: parsePositiveInteger(request.nextUrl.searchParams.get('limit'), 12, 200),
    offset: parseNonNegativeInteger(request.nextUrl.searchParams.get('offset')),
    attacker: parseTextFilter(request.nextUrl.searchParams.get('attacker')),
    victim: parseTextFilter(request.nextUrl.searchParams.get('victim')),
    service: parseTextFilter(request.nextUrl.searchParams.get('service')),
    tick_from: parseNonNegativeInteger(request.nextUrl.searchParams.get('tick_from')),
    tick_to: parseNonNegativeInteger(request.nextUrl.searchParams.get('tick_to')),
  };

  try {
    const data = await listAttackFeed(query);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'attack feed failed';
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

function parseTextFilter(raw: string | null) {
  if (!raw) {
    return undefined;
  }

  const trimmed = raw.trim();
  return trimmed === '' ? undefined : trimmed;
}
