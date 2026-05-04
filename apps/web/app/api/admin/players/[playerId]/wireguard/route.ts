import { handleAdminIdRoute } from '@/lib/api-handler';
import { getAdminPlayerWireGuard } from '@/lib/admin-api';

export async function GET(_request: Request, context: { params: Promise<{ playerId: string }> }) {
  return handleAdminIdRoute(
    context as unknown as { params: Promise<Record<string, string>> },
    'playerId',
    'player id',
    'wireguard config fetch',
    getAdminPlayerWireGuard
  );
}
