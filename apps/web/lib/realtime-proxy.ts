import { firstForwardedAddress } from '@/lib/api-utils';

const streamResponseHeaders = {
  'Content-Type': 'text/event-stream',
  'Cache-Control': 'no-cache, no-transform',
  Connection: 'keep-alive',
  'X-Accel-Buffering': 'no',
};

function trimBaseUrl(value: string) {
  return value.trim().replace(/\/+$/, '');
}

function realtimeBaseUrl() {
  return trimBaseUrl(process.env.AD_PLATFORM_REALTIME_URL ?? 'http://127.0.0.1:8086');
}

function adminRealtimeToken() {
  // Realtime-gateway authenticates admin streams with REALTIME_ADMIN_TOKEN
  // (generated distinctly from ADMIN_API_TOKEN). Fall back for single-token
  // local/dev setups where only ADMIN_API_TOKEN is configured.
  const token =
    process.env.REALTIME_ADMIN_TOKEN?.trim() ||
    process.env.ADMIN_API_TOKEN?.trim();
  if (!token) {
    throw new Error(
      'REALTIME_ADMIN_TOKEN or ADMIN_API_TOKEN must be set for realtime admin proxy calls',
    );
  }
  return token;
}

export async function proxyPublicRealtimeStream(path: string, failureMessage: string, request?: Request) {
  return proxyRealtimeStream(path, failureMessage, request);
}

export async function proxyAdminRealtimeStream(path: string, failureMessage: string, request?: Request) {
  return proxyRealtimeStream(path, failureMessage, request, {
    Authorization: `Bearer ${adminRealtimeToken()}`,
  });
}

async function proxyRealtimeStream(
  path: string,
  failureMessage: string,
  request?: Request,
  headers: HeadersInit = {},
) {
  const forwardedFor = firstForwardedAddress(request?.headers.get('x-forwarded-for') ?? null);
  const upstream = await fetch(`${realtimeBaseUrl()}${path}`, {
    headers: {
      Accept: 'text/event-stream',
      ...(forwardedFor ? { 'X-Forwarded-For': forwardedFor } : {}),
      ...headers,
    },
    cache: 'no-store',
  });

  if (!upstream.ok) {
    const responseHeaders = new Headers({
      'Content-Type': upstream.headers.get('content-type') ?? 'application/json',
    });
    const retryAfter = upstream.headers.get('retry-after');
    if (retryAfter) {
      responseHeaders.set('Retry-After', retryAfter);
    }
    return new Response(
      upstream.body ?? JSON.stringify({ status: 'failed', message: failureMessage }),
      {
        status: upstream.status,
        headers: responseHeaders,
      },
    );
  }

  if (!upstream.body) {
    return new Response(JSON.stringify({ status: 'failed', message: failureMessage }), {
      status: 502,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  return new Response(upstream.body, {
    status: 200,
    headers: streamResponseHeaders,
  });
}
