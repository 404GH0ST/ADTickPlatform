import Link from 'next/link';
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

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="mx-auto flex min-h-screen w-full max-w-md items-center px-4 py-8">
        <div className="grid w-full gap-3">
          <div className="flex justify-end">
            <ThemeToggle />
          </div>
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
