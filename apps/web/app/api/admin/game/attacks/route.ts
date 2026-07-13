import { NextRequest, NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { listAdminGameAttacks } from '@/lib/admin-api';
import { parseAttackFeedQuery } from '@/lib/route-query';

export async function GET(request: NextRequest) {
  try {
    const data = await listAdminGameAttacks(
      parseAttackFeedQuery(request.nextUrl.searchParams),
    );
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'admin attacks failed', 'Upstream request failed');
  }
}
