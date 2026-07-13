import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { stopAdminGameScheduler } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await stopAdminGameScheduler();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'scheduler stop failed', 'Upstream request failed');
  }
}
