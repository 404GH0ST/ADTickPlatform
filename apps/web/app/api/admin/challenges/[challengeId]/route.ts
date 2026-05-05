import { type NextRequest, NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { deleteAdminChallenge, updateAdminChallenge } from '@/lib/admin-api';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ challengeId: string }> },
) {
  try {
    const { challengeId } = await params;
    const data = await deleteAdminChallenge(Number(challengeId));
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'challenge delete failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ challengeId: string }> },
) {
  const body = (await request.json().catch(() => null)) as { name?: string; baseline_image?: string; checker_image?: string; source_bundle_path?: string; weight?: number } | null;
  const name = body?.name?.trim();
  if (!name) {
    return problemResponse(400, 'Invalid request', 'challenge update request is invalid.');
  }
  try {
    const { challengeId } = await params;
    const data = await updateAdminChallenge(Number(challengeId), {
      name,
      baseline_image: body?.baseline_image?.trim() || '',
      checker_image: body?.checker_image?.trim() || '',
      source_bundle_path: body?.source_bundle_path?.trim() || '',
      weight: body?.weight || 1,
    });
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'challenge update failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
