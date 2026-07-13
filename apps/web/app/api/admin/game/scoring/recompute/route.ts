import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { recomputeAdminGameScoring } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await recomputeAdminGameScoring();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'score recompute failed', 'Upstream request failed');
  }
}
