import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { resumeAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await resumeAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'game match resume failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
