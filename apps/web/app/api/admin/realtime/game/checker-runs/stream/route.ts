import { proxyAdminRealtimeStream } from '@/lib/realtime-proxy';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

export async function GET(request: Request) {
  return proxyAdminRealtimeStream(
    '/admin/v1/game/checker-runs/stream',
    'admin realtime checker runs failed', request,
  );
}
