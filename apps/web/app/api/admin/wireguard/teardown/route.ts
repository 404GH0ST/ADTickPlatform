import { NextResponse } from 'next/server';

import { teardownAdminWireGuardGateway } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await teardownAdminWireGuardGateway();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'wireguard gateway teardown failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
