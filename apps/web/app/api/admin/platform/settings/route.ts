import { type NextRequest, NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import {
  getAdminPlatformSettings,
  updateAdminPlatformSettings,
} from '@/lib/admin-api';

export async function GET() {
  try {
    const data = await getAdminPlatformSettings();
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'platform settings fetch failed', 'Upstream request failed');
  }
}

export async function PUT(request: NextRequest) {
  const body = (await request.json().catch(() => null)) as {
    flag_format_prefix?: string;
    max_team_members?: number;
  } | null;
  const prefix = body?.flag_format_prefix?.trim();
  const maxTeamMembers =
    typeof body?.max_team_members === "number" ? body.max_team_members : Number(body?.max_team_members ?? 0);
  if (!prefix || !Number.isInteger(maxTeamMembers) || maxTeamMembers < 0) {
    return problemResponse(400, 'Invalid request', 'flag_format_prefix must not be empty and max_team_members must not be negative.');
  }
  try {
    const data = await updateAdminPlatformSettings({
      flag_format_prefix: prefix,
      max_team_members: maxTeamMembers,
    });
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'platform settings update failed', 'Upstream request failed');
  }
}
