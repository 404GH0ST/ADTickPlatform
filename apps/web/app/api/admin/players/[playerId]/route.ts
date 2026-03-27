import { type NextRequest, NextResponse } from 'next/server';

import { deleteAdminPlayer, updateAdminPlayer } from '@/lib/admin-api';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ playerId: string }> },
) {
  try {
    const { playerId } = await params;
    const data = await deleteAdminPlayer(Number(playerId));
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'player delete failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
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
    return NextResponse.json({ status: 'failed', message: 'player update request is invalid.' }, { status: 400 });
  }
  try {
    const { playerId } = await params;
    const data = await updateAdminPlayer(Number(playerId), { display_name: displayName, email, role });
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'player update failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
