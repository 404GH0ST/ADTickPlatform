import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { reconcileAdminWireGuardGateway } from '@/lib/admin-api';

export async function POST() {
  try {
    const status = await reconcileAdminWireGuardGateway();
    return NextResponse.json(status);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'wireguard gateway reconcile failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
