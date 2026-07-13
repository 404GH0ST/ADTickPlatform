import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { getAdminGameMatchStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameMatchStatus();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game match status failed', 'Upstream request failed');
  }
}
