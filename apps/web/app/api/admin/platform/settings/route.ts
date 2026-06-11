import { type NextRequest, NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import {
  getAdminPlatformSettings,
  updateAdminPlatformSettings,
} from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminPlatformSettings();
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'platform settings fetch failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}

export async function PUT(request: NextRequest) {
  const body = (await request.json().catch(() => null)) as {
    flag_format_prefix?: string;
  } | null;
  const prefix = body?.flag_format_prefix?.trim();
  if (!prefix) {
    return problemResponse(400, 'Invalid request', 'flag_format_prefix must not be empty.');
  }
  try {
    const data = await updateAdminPlatformSettings({ flag_format_prefix: prefix });
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'platform settings update failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
