import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { getAdminOperationsStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminOperationsStatus();
    return NextResponse.json(data);
  } catch (error) {
    const message =
      error instanceof Error ? error.message : 'operations status failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
