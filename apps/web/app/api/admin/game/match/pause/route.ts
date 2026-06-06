import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { pauseAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await pauseAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match pause failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
