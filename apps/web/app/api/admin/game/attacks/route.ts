import { NextRequest, NextResponse } from 'next/server';

import { listAdminGameAttacks } from '@/lib/admin-api';
import { parseAttackFeedQuery } from '@/lib/route-query';

export async function GET(request: NextRequest) {
  try {
    const data = await listAdminGameAttacks(
      parseAttackFeedQuery(request.nextUrl.searchParams),
    );
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'admin attacks failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
