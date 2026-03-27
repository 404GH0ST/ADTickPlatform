import { NextResponse } from 'next/server';

import { validateAdminChallenge } from '@/lib/admin-api';

export async function POST(_request: Request, context: { params: Promise<{ challengeId: string }> }) {
  const { challengeId } = await context.params;
  const challengeID = Number(challengeId);
  if (!Number.isInteger(challengeID) || challengeID <= 0) {
    return NextResponse.json({ status: 'failed', message: 'challenge id is invalid.' }, { status: 400 });
  }

  try {
    const data = await validateAdminChallenge(challengeID);
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'challenge validation failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
