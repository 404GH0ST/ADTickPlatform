import { NextResponse } from 'next/server';

import { getAdminGameStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameStatus();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game-core status failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
