import { NextRequest, NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { listAttackFeed } from '@/lib/platform-api';
import { parseAttackFeedQuery } from '@/lib/route-query';

export async function GET(request: NextRequest) {
  try {
    const data = await listAttackFeed(
      parseAttackFeedQuery(request.nextUrl.searchParams),
    );
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'attack feed failed', 'Upstream request failed');
  }
}
