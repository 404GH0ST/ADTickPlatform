import { NextResponse } from 'next/server';

import { startAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await startAdminGameMatch();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match start failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
