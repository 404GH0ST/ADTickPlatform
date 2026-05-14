import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { auditAdminGameScoring } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await auditAdminGameScoring();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'score audit failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
