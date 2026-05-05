import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { reconcileAdminDeployments } from '@/lib/admin-api';

export async function POST() {
  try {
    const data = await reconcileAdminDeployments();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'deployment reconcile failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
