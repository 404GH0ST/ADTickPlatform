import { NextResponse } from 'next/server';

import { startAdminGameScheduler } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await startAdminGameScheduler();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler start failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
