import { NextResponse } from 'next/server';

import { reconcileAdminWireGuardGateway } from '@/lib/admin-api';

export async function POST() {
  try {
    const status = await reconcileAdminWireGuardGateway();
    return NextResponse.json({ status: 'success', data: status });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'wireguard gateway reconcile failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
