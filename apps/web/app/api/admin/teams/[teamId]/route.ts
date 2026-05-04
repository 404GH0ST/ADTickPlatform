import { type NextRequest, NextResponse } from 'next/server';

import { deleteAdminTeam, updateAdminTeam } from '@/lib/admin-api';
import { parseTeamBody } from '@/lib/api-utils';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ teamId: string }> },
) {
  try {
    const { teamId } = await params;
    const data = await deleteAdminTeam(Number(teamId));
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'team delete failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ teamId: string }> },
) {
  const { name, contactEmail } = await parseTeamBody(request);
  if (!name || !contactEmail) {
    return NextResponse.json({ status: 'failed', message: 'team update request is invalid.' }, { status: 400 });
  }
  try {
    const { teamId } = await params;
    const data = await updateAdminTeam(Number(teamId), { name, contact_email: contactEmail });
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'team update failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
