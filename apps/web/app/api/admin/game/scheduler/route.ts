import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { getAdminGameSchedulerStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameSchedulerStatus();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'scheduler status failed', 'Upstream request failed');
  }
}
