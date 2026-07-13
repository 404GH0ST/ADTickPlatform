import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { teardownAdminWireGuardGateway } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await teardownAdminWireGuardGateway();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'wireguard gateway teardown failed', 'Upstream request failed');
  }
}
