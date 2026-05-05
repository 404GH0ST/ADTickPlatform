import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { getAdminOperationsMetrics } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminOperationsMetrics();
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : 'operations metrics failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
