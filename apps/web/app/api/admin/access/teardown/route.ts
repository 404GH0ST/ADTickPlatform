import { NextResponse } from 'next/server';

import { teardownAdminAccess } from '@/lib/admin-api';
import { upstreamErrorResponse } from '@/lib/api-handler';

export async function POST() {
  try {
    const data = await teardownAdminAccess();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'controller access teardown failed', 'Upstream request failed');
  }
}
