import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { recomputeAdminGameScoring } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await recomputeAdminGameScoring();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'score recompute failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
