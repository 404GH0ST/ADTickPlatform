import { NextRequest, NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { listAttackFeed } from '@/lib/platform-api';
import { parseAttackFeedQuery } from '@/lib/route-query';

export async function GET(request: NextRequest) {
  try {
    const data = await listAttackFeed(
      parseAttackFeedQuery(request.nextUrl.searchParams),
    );
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'attack feed failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
