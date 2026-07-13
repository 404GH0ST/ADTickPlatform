import { NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { createAdminTeam } from '@/lib/admin-api';
import { parseTeamBody } from '@/lib/api-utils';

export async function POST(request: Request) {
  const { name, contactEmail } = await parseTeamBody(request);
  if (!name || !contactEmail) {
    return problemResponse(400, 'Invalid request', 'team request is invalid.');
  }

  try {
    const data = await createAdminTeam({ name, contact_email: contactEmail });
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'team create failed', 'Upstream request failed');
  }
}
