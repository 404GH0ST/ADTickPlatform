import { proxyAdminRealtimeStream } from '@/lib/realtime-proxy';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

export async function GET(request: Request) {
  return proxyAdminRealtimeStream(
    '/admin/v1/game/scheduler/events/stream',
    'admin realtime scheduler events failed', request,
  );
}
