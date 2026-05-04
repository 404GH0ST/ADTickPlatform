import { handleAdminIdRoute } from '@/lib/api-handler';
import { rotateAdminPlayerWireGuard } from '@/lib/admin-api';

export async function POST(_request: Request, context: { params: Promise<{ playerId: string }> }) {
  return handleAdminIdRoute(
    context as unknown as { params: Promise<Record<string, string>> },
    'playerId',
    'player id',
    'wireguard rotate',
    rotateAdminPlayerWireGuard
  );
}
