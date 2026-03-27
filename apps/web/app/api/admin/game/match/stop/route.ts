import { NextResponse } from 'next/server';

import { stopAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await stopAdminGameMatch();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match stop failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
