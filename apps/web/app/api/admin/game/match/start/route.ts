import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { startAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await startAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match start failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
