import { NextResponse } from 'next/server';

import { getAdminAccessStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const status = await getAdminAccessStatus();
    return NextResponse.json({ status: 'success', data: status });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'controller access status failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
