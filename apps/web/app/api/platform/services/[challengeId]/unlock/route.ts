import { NextResponse } from 'next/server';

import { unlockService } from '@/lib/platform-api';

export async function POST(request: Request, context: { params: Promise<{ challengeId: string }> }) {
  const { challengeId } = await context.params;
  const challengeID = Number(challengeId);
  if (!Number.isInteger(challengeID) || challengeID <= 0) {
    return NextResponse.json({ status: 'failed', message: 'challenge id is invalid.' }, { status: 400 });
  }

  const body = (await request.json().catch(() => null)) as { proof?: string } | null;
  const proof = body?.proof?.trim();
  if (!proof) {
    return NextResponse.json({ status: 'failed', message: 'unlock proof is invalid.' }, { status: 400 });
  }

  try {
    const data = await unlockService(challengeID, proof);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'unlock request failed';
    const status = message === 'participant session is not authenticated.' ? 403 : 502;
    return NextResponse.json({ status: status === 403 ? 'forbidden' : 'failed', message }, { status });
  }
}
