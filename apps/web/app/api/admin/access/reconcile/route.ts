import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { reconcileAdminAccess } from '@/lib/admin-api';

export async function POST() {
  try {
    const status = await reconcileAdminAccess();
    return NextResponse.json(status);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'controller access reconcile failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
