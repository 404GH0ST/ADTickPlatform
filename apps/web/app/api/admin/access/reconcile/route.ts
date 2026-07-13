import { NextResponse } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
import { reconcileAdminAccess } from '@/lib/admin-api';

export async function POST() {
  try {
    const status = await reconcileAdminAccess();
    return NextResponse.json(status);
  } catch (error) {
    return upstreamErrorResponse(error, 'controller access reconcile failed', 'Upstream request failed');
  }
}
