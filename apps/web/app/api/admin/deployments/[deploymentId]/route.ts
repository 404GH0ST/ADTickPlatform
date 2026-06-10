import { type NextRequest } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { deleteAdminDeployment } from '@/lib/admin-api';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ deploymentId: string }> },
) {
  try {
    const { deploymentId } = await params;
    await deleteAdminDeployment(Number(deploymentId));
    return new Response(null, { status: 204 });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'deployment delete failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
