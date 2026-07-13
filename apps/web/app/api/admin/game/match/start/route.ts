import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { startAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await startAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game match start failed', 'Upstream request failed');
  }
}
