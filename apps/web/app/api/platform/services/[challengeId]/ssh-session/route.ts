import { handleAdminIdRoute } from '@/lib/api-handler';
import { createSSHSession } from '@/lib/platform-api';

export async function POST(_request: Request, context: { params: Promise<{ challengeId: string }> }) {
  return handleAdminIdRoute(
    context as unknown as { params: Promise<Record<string, string>> },
    'challengeId',
    'challenge id',
    'ssh session creation',
    createSSHSession
  );
}
