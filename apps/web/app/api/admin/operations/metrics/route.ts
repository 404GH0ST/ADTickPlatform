import { NextResponse } from 'next/server';

import { getAdminOperationsMetrics } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminOperationsMetrics();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message =
      error instanceof Error ? error.message : 'operations metrics failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
