import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { listAdminGameSchedulerEvents } from '@/lib/admin-api';
import {
  parseLowercaseTextFilter,
  parseNonNegativeInteger,
  parsePositiveInteger,
} from '@/lib/route-query';

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const query = {
    limit: parsePositiveInteger(searchParams.get('limit'), 12, 200),
    offset: parseNonNegativeInteger(searchParams.get('offset')),
    event_type: parseLowercaseTextFilter(searchParams.get('event_type')),
    source: parseLowercaseTextFilter(searchParams.get('source')),
    state: parseLowercaseTextFilter(searchParams.get('state')),
  };

  try {
    const data = await listAdminGameSchedulerEvents(query);
    return NextResponse.json(data);
  } catch (error) {
    const message = error instanceof Error ? error.message : 'scheduler events failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
