import { NextResponse } from 'next/server';

import { teardownAdminAccess } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await teardownAdminAccess();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'controller access teardown failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
