import { NextResponse } from 'next/server';

import { getAdminWireGuardGatewayStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const status = await getAdminWireGuardGatewayStatus();
    return NextResponse.json({ status: 'success', data: status });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'wireguard gateway status failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
