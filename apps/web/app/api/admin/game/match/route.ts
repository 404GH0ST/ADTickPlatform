import { NextResponse } from 'next/server';

import { getAdminGameMatchStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameMatchStatus();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match status failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
