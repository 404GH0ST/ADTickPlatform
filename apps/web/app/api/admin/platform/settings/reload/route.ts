import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { reloadAdminFlagFormat } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await reloadAdminFlagFormat();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'flag format reload failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
