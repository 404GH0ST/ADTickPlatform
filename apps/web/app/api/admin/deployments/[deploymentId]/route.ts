import { type NextRequest } from 'next/server';

import { upstreamErrorResponse } from '@/lib/api-handler';
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
    return upstreamErrorResponse(error, 'deployment delete failed', 'Upstream request failed');
  }
}
