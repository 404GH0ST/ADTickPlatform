import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { getAdminOperationsStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminOperationsStatus();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'operations status failed');
  }
}
