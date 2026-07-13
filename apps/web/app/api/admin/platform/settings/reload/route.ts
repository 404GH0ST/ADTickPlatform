import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { reloadAdminFlagFormat } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await reloadAdminFlagFormat();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'flag format reload failed', 'Upstream request failed');
  }
}
