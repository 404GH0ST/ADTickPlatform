import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { advanceAdminGameTick } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await advanceAdminGameTick();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game-core tick advance failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
