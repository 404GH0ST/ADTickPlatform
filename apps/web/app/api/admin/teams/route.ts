import { NextResponse } from 'next/server';

import { createAdminTeam } from '@/lib/admin-api';

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as { name?: string; contact_email?: string } | null;
  const name = body?.name?.trim();
  const contactEmail = body?.contact_email?.trim();
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
