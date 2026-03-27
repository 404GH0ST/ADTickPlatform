import { NextResponse } from 'next/server';

import { restartService } from '@/lib/platform-api';

export async function POST(_request: Request, context: { params: Promise<{ challengeId: string }> }) {
  const { challengeId } = await context.params;
  const challengeID = Number(challengeId);
  if (!Number.isInteger(challengeID) || challengeID <= 0) {
    return NextResponse.json({ status: 'failed', message: 'challenge id is invalid.' }, { status: 400 });
  }

  try {
    const data = await restartService(challengeID);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'restart request failed';
    const status = message === 'participant session is not authenticated.' ? 403 : 502;
    return NextResponse.json({ status: status === 403 ? 'forbidden' : 'failed', message }, { status });
  }
}
