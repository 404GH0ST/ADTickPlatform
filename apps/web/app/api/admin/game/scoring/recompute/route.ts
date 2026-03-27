import { NextResponse } from 'next/server';

import { recomputeAdminGameScoring } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await recomputeAdminGameScoring();
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'score recompute failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
