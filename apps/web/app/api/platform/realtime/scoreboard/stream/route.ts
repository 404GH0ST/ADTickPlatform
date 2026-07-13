import { proxyPublicRealtimeStream } from '@/lib/realtime-proxy';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

export async function GET(request: Request) {
  return proxyPublicRealtimeStream(
    '/public/v1/scoreboard/stream',
    'participant realtime scoreboard failed', request,
  );
}
