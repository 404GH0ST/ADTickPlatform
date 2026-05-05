import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { getAdminAccessStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const status = await getAdminAccessStatus();
    return NextResponse.json(status);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'controller access status failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
