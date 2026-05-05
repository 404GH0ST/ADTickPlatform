import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { startAdminGameScheduler } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await startAdminGameScheduler();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler start failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
