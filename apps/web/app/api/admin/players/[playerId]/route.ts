import { type NextRequest, NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { deleteAdminPlayer, updateAdminPlayer } from '@/lib/admin-api';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ playerId: string }> },
) {
  try {
    const { playerId } = await params;
    await deleteAdminPlayer(Number(playerId));
    return new Response(null, { status: 204 });
  } catch (error) {
    return upstreamErrorResponse(error, 'player delete failed', 'Upstream request failed');
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ playerId: string }> },
) {
  const body = (await request.json().catch(() => null)) as { display_name?: string; email?: string; role?: string } | null;
  const displayName = body?.display_name?.trim();
  const email = body?.email?.trim();
  const role = body?.role?.trim() || 'participant';
  if (!displayName || !email) {
    return problemResponse(400, 'Invalid request', 'player update request is invalid.');
  }
  try {
    const { playerId } = await params;
    const data = await updateAdminPlayer(Number(playerId), { display_name: displayName, email, role });
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'player update failed', 'Upstream request failed');
  }
}
