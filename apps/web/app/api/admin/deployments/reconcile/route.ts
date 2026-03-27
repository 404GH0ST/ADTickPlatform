import { NextResponse } from 'next/server';

import { reconcileAdminDeployments } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await reconcileAdminDeployments();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'deployment reconcile failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
