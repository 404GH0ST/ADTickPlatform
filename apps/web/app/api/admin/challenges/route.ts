import { NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { createAdminChallenge } from '@/lib/admin-api';

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    name?: string;
    baseline_image?: string;
    checker_image?: string;
    source_bundle_path?: string;
    service_port?: number;
    service_subnet_octet?: number;
    egress_enabled?: boolean;
    category?: string;
    protocol?: string;
  } | null;

  if (!body?.name?.trim()) {
    return problemResponse(400, 'Invalid request', 'challenge request is invalid.');
  }

  try {
    const data = await createAdminChallenge({
      name: body.name.trim(),
      baseline_image: body.baseline_image?.trim() || '',
      checker_image: body.checker_image?.trim() || '',
      source_bundle_path: body.source_bundle_path?.trim() || '',
      service_port: Number(body.service_port) || undefined,
      service_subnet_octet: Number(body.service_subnet_octet) || undefined,
      egress_enabled: typeof body.egress_enabled === 'boolean' ? body.egress_enabled : undefined,
      // The gateway validates both and answers 400 with the reason.
      category: typeof body.category === 'string' ? body.category : undefined,
      protocol: typeof body.protocol === 'string' ? body.protocol : undefined,
    });
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'challenge create failed', 'Upstream request failed');
  }
}
