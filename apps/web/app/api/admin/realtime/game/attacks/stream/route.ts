import { proxyPublicRealtimeStream } from '@/lib/realtime-proxy';

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

export async function GET(request: Request) {
  return proxyPublicRealtimeStream('/public/v1/attacks/stream', 'admin realtime attacks failed', request);
}
