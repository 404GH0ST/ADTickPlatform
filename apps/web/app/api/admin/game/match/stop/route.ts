import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { stopAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await stopAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game match stop failed', 'Upstream request failed');
  }
}
