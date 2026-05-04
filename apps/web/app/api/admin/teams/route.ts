import { NextResponse } from 'next/server';

import { createAdminTeam } from '@/lib/admin-api';
import { parseTeamBody } from '@/lib/api-utils';

export async function POST(request: Request) {
  const { name, contactEmail } = await parseTeamBody(request);
  if (!name || !contactEmail) {
    return NextResponse.json({ status: 'failed', message: 'team request is invalid.' }, { status: 400 });
  }

  try {
    const data = await createAdminTeam({ name, contact_email: contactEmail });
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'team create failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
