import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { startAdminGameScheduler } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await startAdminGameScheduler();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'scheduler start failed', 'Upstream request failed');
  }
}
