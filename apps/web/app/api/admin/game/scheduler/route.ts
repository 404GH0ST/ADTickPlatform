import { NextResponse } from 'next/server';

import { getAdminGameSchedulerStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameSchedulerStatus();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler status failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
