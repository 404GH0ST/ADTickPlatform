import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { getAdminGameSchedulerStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameSchedulerStatus();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler status failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
