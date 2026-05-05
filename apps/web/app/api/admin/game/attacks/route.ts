import { NextRequest, NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { listAdminGameAttacks } from '@/lib/admin-api';
import { parseAttackFeedQuery } from '@/lib/route-query';

export async function GET(request: NextRequest) {
  try {
    const data = await listAdminGameAttacks(
      parseAttackFeedQuery(request.nextUrl.searchParams),
    );
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'admin attacks failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
