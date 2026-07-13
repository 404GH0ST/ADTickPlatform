import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { listAdminDeployments } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await listAdminDeployments();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'deployment list failed', 'Upstream request failed');
  }
}
