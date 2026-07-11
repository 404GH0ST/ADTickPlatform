import { redirect } from 'next/navigation';

import { ParticipantLoginForm } from '@/components/dashboard/participant-login-form';
import { AppearanceControls } from '@/components/ui/appearance-controls';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { WorkspaceFrame } from '@/components/ui/workspace-frame';
import { getParticipantSession } from '@/lib/platform-api';

export const dynamic = 'force-dynamic';

export default async function ParticipantLoginPage() {
  const session = await getParticipantSession();
  if (session.authenticated) {
    redirect('/services');
  }

  const deactivated = session.reason === 'deactivated';

  return (
    <main className="workspace-canvas flex items-center justify-center text-foreground">
      <WorkspaceFrame className="min-h-0 w-full max-w-md" bodyClassName="gap-3">
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
        <Card className="w-full border-border bg-background shadow-none">
          <CardHeader>
            <CardTitle>Sign In</CardTitle>
            <CardDescription>
              Sign in with your team member account.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <ParticipantLoginForm />
          </CardContent>
        </Card>
      </WorkspaceFrame>
    </main>
  );
}
