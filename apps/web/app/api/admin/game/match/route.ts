import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { getAdminGameMatchStatus } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminGameMatchStatus();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match status failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
