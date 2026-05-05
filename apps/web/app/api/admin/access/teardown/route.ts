import { NextResponse } from 'next/server';

import { teardownAdminAccess } from '@/lib/admin-api';
import { problemResponse } from '@/lib/api-handler';

export async function POST() {
  try {
    const data = await teardownAdminAccess();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'controller access teardown failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
