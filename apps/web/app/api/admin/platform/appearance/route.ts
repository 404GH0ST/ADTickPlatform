import { type NextRequest, NextResponse } from 'next/server';

import { problemResponse, upstreamErrorResponse } from '@/lib/api-handler';
import { updateAdminPlatformAppearance } from '@/lib/admin-api';
import { setCachedActiveThemeId } from '@/lib/platform-appearance';
import { getThemes, isKnownThemeId } from '@/lib/themes';

export async function PUT(request: NextRequest) {
  const body = (await request.json().catch(() => null)) as { theme_id?: unknown } | null;
  const themeID = typeof body?.theme_id === 'string' ? body.theme_id.trim() : '';
  // The web app owns the theme catalogue, so only ids with a themes/<id>/ folder are accepted.
  if (!isKnownThemeId(themeID)) {
    return problemResponse(
      400,
      'Invalid request',
      `theme_id must be one of: ${getThemes().map((theme) => theme.id).join(', ')}.`,
    );
  }
  try {
    const data = await updateAdminPlatformAppearance(themeID);
    setCachedActiveThemeId(data.theme_id);
    return NextResponse.json(data);
  } catch (error) {
    return upstreamErrorResponse(error, 'platform appearance update failed', 'Upstream request failed');
  }
}
