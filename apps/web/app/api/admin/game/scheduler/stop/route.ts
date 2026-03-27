import { NextResponse } from 'next/server';

import { stopAdminGameScheduler } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await stopAdminGameScheduler();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler stop failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
