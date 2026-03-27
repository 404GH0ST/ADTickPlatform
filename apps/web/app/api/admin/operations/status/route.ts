import { NextResponse } from 'next/server';

import { getAdminOperationsStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminOperationsStatus();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message =
      error instanceof Error ? error.message : 'operations status failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
