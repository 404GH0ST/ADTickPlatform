import { type NextRequest, NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { deleteAdminTeam, updateAdminTeam } from '@/lib/admin-api';
import { parseTeamBody } from '@/lib/api-utils';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ teamId: string }> },
) {
  try {
    const { teamId } = await params;
    await deleteAdminTeam(Number(teamId));
    return new Response(null, { status: 204 });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'team delete failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ teamId: string }> },
) {
  const { name, contactEmail } = await parseTeamBody(request);
  if (!name || !contactEmail) {
    return problemResponse(400, 'Invalid request', 'team update request is invalid.');
  }
  try {
    const { teamId } = await params;
    const data = await updateAdminTeam(Number(teamId), { name, contact_email: contactEmail });
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'team update failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
