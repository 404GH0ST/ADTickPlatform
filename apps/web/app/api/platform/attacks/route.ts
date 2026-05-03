import { NextRequest, NextResponse } from 'next/server';

import { listAttackFeed } from '@/lib/platform-api';
import { parseAttackFeedQuery } from '@/lib/route-query';

export async function GET(request: NextRequest) {
  try {
    const data = await listAttackFeed(
      parseAttackFeedQuery(request.nextUrl.searchParams),
    );
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'attack feed failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
