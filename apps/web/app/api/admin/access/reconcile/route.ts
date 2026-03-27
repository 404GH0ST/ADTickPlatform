import { NextResponse } from 'next/server';

import { reconcileAdminAccess } from '@/lib/admin-api';

export async function POST() {
  try {
    const status = await reconcileAdminAccess();
    return NextResponse.json({ status: 'success', data: status });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'controller access reconcile failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
