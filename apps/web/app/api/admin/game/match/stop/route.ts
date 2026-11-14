import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { getAdminGameMatchStatus, stopAdminGameMatch } from '@/lib/admin-api';
import { stopMatchTolerantOfTimeouts } from '@/lib/match-stop';

export async function POST() {
  try {
    const data = await stopMatchTolerantOfTimeouts(stopAdminGameMatch, getAdminGameMatchStatus);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game match stop failed', 'Upstream request failed');
  }
}
