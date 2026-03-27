import { type NextRequest, NextResponse } from 'next/server';

import { deleteAdminDeployment } from '@/lib/admin-api';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ deploymentId: string }> },
) {
  try {
    const { deploymentId } = await params;
    const data = await deleteAdminDeployment(Number(deploymentId));
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'deployment delete failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
