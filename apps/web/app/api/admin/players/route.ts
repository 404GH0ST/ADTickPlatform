import { NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { createAdminPlayer } from '@/lib/admin-api';

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    team_id?: number;
    display_name?: string;
    email?: string;
    password?: string;
    role?: string;
  } | null;

  if (
    !body?.team_id ||
    !body.display_name?.trim() ||
    !body.email?.trim() ||
    !body.password ||
    body.password.length < 8 ||
    !body.password.trim()
  ) {
    return problemResponse(400, 'Invalid request', 'player request is invalid.');
  }

  try {
    const data = await createAdminPlayer({
      team_id: body.team_id,
      display_name: body.display_name.trim(),
      email: body.email.trim(),
      password: body.password,
      role: body.role?.trim() || 'member',
    });
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'player create failed', 'Upstream request failed');
  }
}
