import { NextResponse } from 'next/server';

import { problemResponse } from '@/lib/api-handler';
import { adminProxyFetch } from '@/lib/admin-api';

export async function POST() {
  try {
    const response = await adminProxyFetch("/api/v2/admin/deployments/reconcile", {
      method: "POST",
    });
    const body = await response.text();
    return new NextResponse(body, {
      status: response.status,
      headers: {
        "content-type":
          response.headers.get("content-type") ?? "application/json",
      },
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : 'deployment reconcile failed';
    return problemResponse(502, 'Upstream request failed', message);
  }
}
