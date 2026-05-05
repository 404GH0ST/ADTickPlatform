import { NextResponse } from 'next/server';

import { unlockService } from '@/lib/platform-api';

function problemResponse(status: number, title: string, detail: string) {
  return NextResponse.json(
    { title, status, detail },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  );
}

export async function POST(request: Request, context: { params: Promise<{ challengeId: string }> }) {
  const { challengeId } = await context.params;
  const challengeID = Number(challengeId);
  if (!Number.isInteger(challengeID) || challengeID <= 0) {
    return problemResponse(400, 'Invalid request', 'challenge id is invalid.');
  }

  const body = (await request.json().catch(() => null)) as { proof?: string } | null;
  const proof = body?.proof?.trim();
  if (!proof) {
    return problemResponse(400, 'Invalid request', 'unlock proof is invalid.');
  }

  try {
    const data = await unlockService(challengeID, proof);
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'unlock request failed';
    const status = message === 'participant session is not authenticated.' ? 403 : 502;
    return problemResponse(status, status === 403 ? 'Authentication required' : 'Unlock request failed', message);
  }
}
