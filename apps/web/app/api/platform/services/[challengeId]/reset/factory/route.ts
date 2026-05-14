import { handleParticipantIdRoute } from '@/lib/api-handler';
import { factoryResetService } from '@/lib/platform-api';

export async function POST(_request: Request, context: { params: Promise<{ challengeId: string }> }) {
  return handleParticipantIdRoute(
    context as unknown as { params: Promise<Record<string, string>> },
    'challengeId',
    'challenge id',
    'factory reset',
    factoryResetService
  );
}
