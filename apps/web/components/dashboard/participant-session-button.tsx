'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { clearFlagDraft } from '@/lib/flag-draft';

export function ParticipantLogoutButton() {
  const router = useRouter();
  const [pending, setPending] = useState(false);

  async function logout() {
    setPending(true);
    try {
      await fetch('/api/platform/session/logout', {
        method: 'POST',
      });
      clearFlagDraft();
      router.push('/');
      router.refresh();
    } finally {
      setPending(false);
    }
  }

  return (
    <Button type="button" variant="outline" onClick={logout} disabled={pending}>
      {pending ? 'Signing Out...' : 'Sign Out'}
    </Button>
  );
}
