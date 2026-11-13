import { handleAdminIdRoute } from '@/lib/api-handler';
import { deleteAdminSubmission } from '@/lib/admin-api';

export async function DELETE(
  _request: Request,
  context: { params: Promise<{ submissionId: string }> },
) {
  return handleAdminIdRoute(
    context,
    'submissionId',
    'Submission ID',
    'submission deletion',
    deleteAdminSubmission,
  );
}
