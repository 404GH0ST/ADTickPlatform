import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { advanceAdminGameTick } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await advanceAdminGameTick();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game-core tick advance failed', 'Upstream request failed');
  }
}
