import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { teardownAdminWireGuardGateway } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await teardownAdminWireGuardGateway();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'wireguard gateway teardown failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
