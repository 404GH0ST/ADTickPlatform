import { proxyAdminRealtimeStream } from '@/lib/realtime-proxy';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

export async function GET() {
  return proxyAdminRealtimeStream(
    '/admin/v1/game/scoreboard/stream',
    'admin realtime scoreboard failed',
  );
}
