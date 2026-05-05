import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { listAdminGameScoreboard } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await listAdminGameScoreboard();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scoreboard fetch failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
