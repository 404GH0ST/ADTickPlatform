'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { clearFlagDraft } from '@/lib/flag-draft';

export function ParticipantLogoutButton() {
  const router = useRouter();
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);

  async function logout() {
    setPending(true);
    setFailed(false);
    try {
      const response = await fetch('/api/platform/session/logout', {
        method: 'POST',
      });
      if (!response.ok) {
        setFailed(true);
        return;
      }
      clearFlagDraft();
      router.push('/');
      router.refresh();
    } catch {
      setFailed(true);
    } finally {
      setPending(false);
    }
  }

  return (
    <>
      <Button type="button" variant="outline" onClick={logout} disabled={pending}>
        {pending ? 'Signing Out...' : 'Sign Out'}
      </Button>
      {failed && <span role="alert">Couldn’t sign out. Try again.</span>}
    </>
  );
}
