import { NextResponse } from 'next/server';

import { createAdminPlayer } from '@/lib/admin-api';

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    team_id?: number;
    display_name?: string;
    email?: string;
    password?: string;
    role?: string;
  } | null;

  if (!body?.team_id || !body.display_name?.trim() || !body.email?.trim() || !body.password?.trim()) {
    return NextResponse.json({ status: 'failed', message: 'player request is invalid.' }, { status: 400 });
  }

  try {
    const data = await createAdminPlayer({
      team_id: body.team_id,
      display_name: body.display_name.trim(),
      email: body.email.trim(),
      password: body.password.trim(),
      role: body.role?.trim() || 'member',
    });
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'player create failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
