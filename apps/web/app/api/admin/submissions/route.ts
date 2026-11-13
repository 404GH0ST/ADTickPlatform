import { NextRequest, NextResponse } from 'next/server';
import { upstreamErrorResponse } from '@/lib/api-handler';
import { listAdminSubmissions } from '@/lib/admin-api';
import { parseAttackFeedQuery } from '@/lib/route-query';

export async function GET(request: NextRequest) {
  try {
    const params = request.nextUrl.searchParams;
    return NextResponse.json(
      await listAdminSubmissions({
        ...parseAttackFeedQuery(params, 25),
        status: params.get('status')?.trim() || undefined,
        flag: params.get('flag')?.trim() || undefined,
      }),
    );
  } catch (error) {
    return upstreamErrorResponse(
      error,
      'submissions unavailable',
      'Could not load submissions.',
    );
  }
}
