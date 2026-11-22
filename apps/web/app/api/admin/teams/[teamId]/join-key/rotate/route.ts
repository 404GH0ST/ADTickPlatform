import { handleAdminIdRoute } from '@/lib/api-handler';
import { rotateAdminTeamJoinKey } from '@/lib/admin-api';

export async function POST(_request: Request, context: { params: Promise<{ teamId: string }> }) {
  const response = await handleAdminIdRoute(
    context, 'teamId', 'team id', 'rotate join key', rotateAdminTeamJoinKey,
  );
  response.headers.set('Cache-Control', 'no-store');
  return response;
}
