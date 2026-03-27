import { NextResponse } from 'next/server';

import { listAdminGameScoreboard } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await listAdminGameScoreboard();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scoreboard fetch failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
