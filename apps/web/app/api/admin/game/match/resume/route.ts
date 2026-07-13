import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { resumeAdminGameMatch } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await resumeAdminGameMatch();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'game match resume failed', 'Upstream request failed');
  }
}
