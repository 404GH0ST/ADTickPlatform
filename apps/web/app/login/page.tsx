import type { Metadata } from 'next';
import { redirect } from 'next/navigation';

import { ParticipantLoginCard } from '@/components/dashboard/participant-login-form';
import { AppearanceControls } from '@/components/ui/appearance-controls';
import { WorkspaceFrame } from '@/components/ui/workspace-frame';
import { getParticipantSession } from '@/lib/platform-api';

export const metadata: Metadata = { title: 'Sign In' };

export const dynamic = 'force-dynamic';

export default async function ParticipantLoginPage() {
  const session = await getParticipantSession();
  if (session.authenticated) {
    redirect('/services');
  }

  const deactivated = session.reason === 'deactivated';
  const unavailable = session.reason === 'unavailable';

  return (
    <main className="workspace-canvas flex items-start justify-center text-foreground">
      <WorkspaceFrame className="workspace-frame-auth min-h-0 w-full max-w-md" bodyClassName="gap-3">
        <div className="flex justify-end">
          <AppearanceControls />
        </div>
        {deactivated && (
          <div
            role="alert"
            data-testid="deactivated-notice"
            className="tone-danger rounded-md border px-3 py-2 text-sm"
          >
            Your account or team has been deactivated by the organizers. Contact
            them if you believe this is a mistake.
          </div>
        )}
        {unavailable && (
          <div
            role="alert"
            data-testid="session-unavailable-notice"
            className="tone-warning rounded-md border px-3 py-2 text-sm"
          >
            The platform API can&apos;t be reached, so your session couldn&apos;t be checked.
            You have not been signed out. Try again in a moment.
          </div>
        )}
        <ParticipantLoginCard />
      </WorkspaceFrame>
    </main>
  );
}
