import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { auditAdminGameScoring } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await auditAdminGameScoring();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'score audit failed', 'Upstream request failed');
  }
}
