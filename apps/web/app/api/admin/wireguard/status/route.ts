import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { getAdminWireGuardGatewayStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const status = await getAdminWireGuardGatewayStatus();
    return NextResponse.json(status);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'wireguard gateway status failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
