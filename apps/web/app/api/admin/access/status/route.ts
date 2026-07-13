import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { getAdminAccessStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const status = await getAdminAccessStatus();
    return NextResponse.json(status);
  } catch (error) {
    return upstreamErrorResponse(error, 'controller access status failed', 'Upstream request failed');
  }
}
