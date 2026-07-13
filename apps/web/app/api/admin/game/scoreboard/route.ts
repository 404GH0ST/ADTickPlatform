import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { listAdminGameScoreboard } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await listAdminGameScoreboard();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'scoreboard fetch failed', 'Upstream request failed');
  }
}
