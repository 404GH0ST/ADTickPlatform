import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { getAdminGameStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameStatus();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game-core status failed', 'Upstream request failed');
  }
}
