import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { stopAdminGameScheduler } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await stopAdminGameScheduler();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler stop failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
