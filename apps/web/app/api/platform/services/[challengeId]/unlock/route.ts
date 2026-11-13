import { NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { unlockService } from '@/lib/platform-api';

export async function POST(_request: Request, context: { params: Promise<{ challengeId: string }> }) {
  const { challengeId } = await context.params;
  const challengeID = Number(challengeId);
  if (!Number.isInteger(challengeID) || challengeID <= 0) {
    return problemResponse(400, 'Invalid request', 'challenge id is invalid.');
  }

  try {
    const data = await unlockService(challengeID);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'unlock request failed', 'Unlock request failed');
  }
}
