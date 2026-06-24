import { handleAdminIdRoute } from '@/lib/api-handler';
import { deactivateAdminTeam } from '@/lib/admin-api';

export async function POST(_request: Request, context: { params: Promise<{ teamId: string }> }) {
  return handleAdminIdRoute(
    context as unknown as { params: Promise<Record<string, string>> },
    'teamId',
    'team id',
    'team deactivate',
    deactivateAdminTeam,
  );
}
