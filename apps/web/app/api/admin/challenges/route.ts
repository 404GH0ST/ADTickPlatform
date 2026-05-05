import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { createAdminChallenge } from '@/lib/admin-api';

export async function POST(request: Request) {
  const body = (await request.json().catch(() => null)) as {
    name?: string;
    baseline_image?: string;
    checker_image?: string;
    source_bundle_path?: string;
    weight?: number;
    service_port?: number;
    service_subnet_octet?: number;
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
      weight: Number(body.weight) || 1,
      service_port: Number(body.service_port) || undefined,
      service_subnet_octet: Number(body.service_subnet_octet) || undefined,
    });
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'challenge create failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
