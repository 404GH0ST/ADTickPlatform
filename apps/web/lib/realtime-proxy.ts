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

function adminToken() {
  const token = process.env.ADMIN_API_TOKEN?.trim();
  if (!token) {
    throw new Error('ADMIN_API_TOKEN must be set for realtime admin proxy calls');
  }
  return token;
}

export async function proxyPublicRealtimeStream(path: string, failureMessage: string) {
  return proxyRealtimeStream(path, failureMessage);
}

export async function proxyAdminRealtimeStream(path: string, failureMessage: string) {
  return proxyRealtimeStream(path, failureMessage, {
    Authorization: `Bearer ${adminToken()}`,
  });
}

async function proxyRealtimeStream(
  path: string,
  failureMessage: string,
  headers: HeadersInit = {},
) {
  const upstream = await fetch(`${realtimeBaseUrl()}${path}`, {
    headers: {
      Accept: 'text/event-stream',
      ...headers,
    },
    cache: 'no-store',
  });

  if (!upstream.ok || !upstream.body) {
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
