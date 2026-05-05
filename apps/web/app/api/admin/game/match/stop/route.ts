import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { stopAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await stopAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match stop failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
