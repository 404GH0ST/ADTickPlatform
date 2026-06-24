import { redirect } from 'next/navigation';

import { ParticipantLoginForm } from '@/components/dashboard/participant-login-form';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { getParticipantSession } from '@/lib/platform-api';

export const dynamic = 'force-dynamic';

export default async function ParticipantLoginPage() {
  const session = await getParticipantSession();
  if (session.authenticated) {
    redirect('/services');
  }

  const deactivated = session.reason === 'deactivated';

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex min-h-screen w-full max-w-md items-center px-4 py-8">
        <div className="grid w-full gap-3">
          <div className="flex justify-end">
            <ThemeToggle />
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
          <Card className="w-full">
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
        </div>
      </div>
    </main>
  );
}
