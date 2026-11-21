import { type NextRequest, NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { deleteAdminChallenge, updateAdminChallenge } from '@/lib/admin-api';

export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ challengeId: string }> },
) {
  try {
    const { challengeId } = await params;
    await deleteAdminChallenge(Number(challengeId));
    return new Response(null, { status: 204 });
  } catch (error) {
    return upstreamErrorResponse(error, 'challenge delete failed', 'Upstream request failed');
  }
}

export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ challengeId: string }> },
) {
  const body = (await request.json().catch(() => null)) as {
    name?: string;
    baseline_image?: string;
    checker_image?: string;
    source_bundle_path?: string;
    egress_enabled?: boolean;
    category?: string;
    protocol?: string;
  } | null;
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
      egress_enabled: typeof body?.egress_enabled === 'boolean' ? body.egress_enabled : undefined,
      // Omitted fields keep the stored values; the gateway validates sent ones.
      category: typeof body?.category === 'string' ? body.category : undefined,
      protocol: typeof body?.protocol === 'string' ? body.protocol : undefined,
    });
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'challenge update failed', 'Upstream request failed');
  }
}
