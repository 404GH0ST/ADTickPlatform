import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { getAdminWireGuardGatewayStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const status = await getAdminWireGuardGatewayStatus();
    return NextResponse.json(status);
  } catch (error) {
    return upstreamErrorResponse(error, 'wireguard gateway status failed', 'Upstream request failed');
  }
}
