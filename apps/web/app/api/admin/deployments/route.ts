import { NextResponse } from 'next/server';

import { listAdminDeployments } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await listAdminDeployments();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'deployment list failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
