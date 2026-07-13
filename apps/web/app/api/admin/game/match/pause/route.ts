import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { pauseAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await pauseAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game match pause failed', 'Upstream request failed');
  }
}
