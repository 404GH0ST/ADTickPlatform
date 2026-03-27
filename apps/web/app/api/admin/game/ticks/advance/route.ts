import { NextResponse } from 'next/server';

import { advanceAdminGameTick } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await advanceAdminGameTick();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game-core tick advance failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
