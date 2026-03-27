import { type NextRequest, NextResponse } from 'next/server';

import { deleteAdminChallenge, updateAdminChallenge } from '@/lib/admin-api';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ challengeId: string }> },
) {
  try {
    const { challengeId } = await params;
    const data = await deleteAdminChallenge(Number(challengeId));
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'challenge delete failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ challengeId: string }> },
) {
  const body = (await request.json().catch(() => null)) as { name?: string; baseline_image?: string; checker_image?: string; weight?: number } | null;
  const name = body?.name?.trim();
  if (!name) {
    return NextResponse.json({ status: 'failed', message: 'challenge update request is invalid.' }, { status: 400 });
  }
  try {
    const { challengeId } = await params;
    const data = await updateAdminChallenge(Number(challengeId), {
      name,
      baseline_image: body?.baseline_image?.trim() || '',
      checker_image: body?.checker_image?.trim() || '',
      weight: body?.weight || 1,
    });
    return NextResponse.json({ status: 'success', data });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'challenge update failed';
    return NextResponse.json({ status: 'failed', message }, { status: 502 });
  }
}
