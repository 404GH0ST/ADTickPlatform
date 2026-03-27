import { NextResponse } from 'next/server';

import { revokeAdminPlayerWireGuard } from '@/lib/admin-api';

export async function POST(_request: Request, context: { params: Promise<{ playerId: string }> }) {
  const { playerId } = await context.params;
  const playerID = Number(playerId);
  if (!Number.isInteger(playerID) || playerID <= 0) {
    return NextResponse.json({ status: 'failed', message: 'player id is invalid.' }, { status: 400 });
  }

  try {
    const data = await revokeAdminPlayerWireGuard(playerID);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'wireguard revoke failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
