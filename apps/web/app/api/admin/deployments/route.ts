import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { listAdminDeployments } from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await listAdminDeployments();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'deployment list failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
