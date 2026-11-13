'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { RefreshCw } from 'lucide-react';
import type {
  AdminSubmission,
  AdminSubmissionPage,
  SubmissionDeletion,
} from '@/lib/admin-dashboard-types';
import { processApiResponse } from '@/lib/api-utils';
import { formatIndonesianDate } from '@/lib/date-format';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { StatusBanner } from '@/components/ui/status-banner';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

export function RefreshSubmissions() {
  const router = useRouter();
  return (
    <Button variant="outline" onClick={() => router.refresh()}>
      <RefreshCw className="h-4 w-4" />
      Refresh
    </Button>
  );
}

export function SubmissionsTable({
  page,
  error,
}: {
  page: AdminSubmissionPage;
  error?: string;
}) {
  const router = useRouter();
  const [selected, setSelected] = useState<AdminSubmission | null>(null);
  const [pending, setPending] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [removed, setRemoved] = useState<Set<number>>(() => new Set());

  async function removeSubmission() {
    if (!selected || pending) return;
    setPending(true);
    setActionError(null);
    try {
      const response = await fetch(`/api/admin/submissions/${selected.id}`, {
        method: 'DELETE',
      });
      const result = await processApiResponse<SubmissionDeletion>(
        response,
        'Delete submission',
      );
      setRemoved((ids) => new Set([...ids, selected.id]));
      setNote(
        result.first_blood_cleared
          ? `Submission #${result.id} deleted. First blood on ${result.service} is open for the next accepted capture.`
          : `Submission #${result.id} deleted.${result.status === 'accepted' ? ' Its capture has been removed from scoring and the attack feed.' : ' Scores are unchanged.'}`,
      );
      setSelected(null);
      if (page.items.length === 1 && page.offset > 0) {
        const params = new URLSearchParams(window.location.search);
        params.set('offset', String(Math.max(0, page.offset - page.limit)));
        router.replace(`/admin/submissions?${params}`);
      } else {
        router.refresh();
      }
    } catch (cause) {
      setActionError(
        cause instanceof Error
          ? cause.message
          : 'Could not delete this submission. Try again.',
      );
    } finally {
      setPending(false);
    }
  }

  const rows = page.items.filter((row) => !removed.has(row.id));
  return (
    <>
      {error ? (
        <div className="p-4">
          <StatusBanner variant="error" message={error} />
        </div>
      ) : null}
      {note ? (
        <div className="border-b p-4">
          <StatusBanner variant="success" message={note} />
        </div>
      ) : null}
      <Table data-testid="submissions-table" className="min-w-[68rem]">
        <TableHeader>
          <TableRow>
            <TableHead>Submitted</TableHead>
            <TableHead>Tick</TableHead>
            <TableHead>Submitting team</TableHead>
            <TableHead>Victim</TableHead>
            <TableHead>Challenge</TableHead>
            <TableHead>Flag</TableHead>
            <TableHead>Verdict</TableHead>
            <TableHead className="text-right">Action</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={8}
                className="py-10 text-center text-muted-foreground"
              >
                {error
                  ? 'Submission history is unavailable. Use Refresh to try again.'
                  : 'No submissions match this view.'}
              </TableCell>
            </TableRow>
          ) : (
            rows.map((row) => (
              <TableRow key={row.id} data-testid={`submission-${row.id}`}>
                <TableCell className="whitespace-nowrap text-xs">
                  <time dateTime={row.submitted_at}>
                    {formatIndonesianDate(row.submitted_at)}
                  </time>
                  <span className="mt-0.5 block text-muted-foreground">
                    #{row.id}
                  </span>
                </TableCell>
                <TableCell className="font-mono text-xs">#{row.tick}</TableCell>
                <TableCell>{row.attacker}</TableCell>
                <TableCell>{row.victim || '—'}</TableCell>
                <TableCell>
                  {row.service || '—'}
                  {row.first_blood ? (
                    <span className="medal-gold mt-1 block w-fit rounded-sm border px-1.5 py-0.5 text-xs font-medium">
                      First blood
                    </span>
                  ) : null}
                </TableCell>
                <TableCell className="max-w-[18rem]">
                  <details>
                    <summary className="cursor-pointer break-all font-mono text-xs focus-visible:outline-2 focus-visible:outline-ring">
                      {row.flag.length > 30
                        ? `${row.flag.slice(0, 27)}…`
                        : row.flag || '(empty flag)'}
                    </summary>
                    <p className="mt-2 break-all font-mono text-xs">
                      {row.flag || '(empty flag)'}
                    </p>
                  </details>
                </TableCell>
                <TableCell className="max-w-[17rem]">
                  <span
                    className={`inline-block rounded-sm border px-1.5 py-0.5 text-xs font-medium ${row.status === 'accepted' ? 'tone-success' : row.status === 'invalid' ? 'tone-danger' : 'tone-neutral'}`}
                  >
                    {row.status === 'invalid'
                      ? 'Rejected'
                      : row.status === 'duplicate'
                        ? 'Duplicate'
                        : 'Accepted'}
                  </span>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {row.detail}
                  </p>
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="touch"
                    aria-label={`Delete submission #${row.id}`}
                    onClick={() => {
                      setActionError(null);
                      setSelected(row);
                    }}
                  >
                    Delete
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
      <Dialog
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open && !pending) setSelected(null);
        }}
      >
        <DialogContent showCloseButton={!pending}>
          <DialogHeader>
            <DialogTitle>Delete submission #{selected?.id}?</DialogTitle>
            <DialogDescription>
              {selected?.status === 'accepted'
                ? 'This removes the accepted capture from the attack feed and scoring. The flag can be submitted again if it is still valid.'
                : 'This removes only this attempt from submission history. It does not change scores or an existing capture.'}
            </DialogDescription>
          </DialogHeader>
          {selected ? (
            <dl className="grid grid-cols-[6rem_1fr] gap-x-4 gap-y-2 border-y py-3 text-sm">
              <dt className="text-muted-foreground">Team</dt>
              <dd className="break-words">{selected.attacker}</dd>
              <dt className="text-muted-foreground">Challenge</dt>
              <dd className="break-words">{selected.service || 'Unknown'}</dd>
              <dt className="text-muted-foreground">Tick</dt>
              <dd>#{selected.tick}</dd>
            </dl>
          ) : null}
          {selected?.first_blood ? (
            <StatusBanner
              variant="warning"
              message={`First blood on ${selected.service} will be cleared. The next accepted capture earns it; existing captures will not be promoted.`}
            />
          ) : null}
          {actionError ? (
            <StatusBanner variant="error" message={actionError} />
          ) : null}
          <div className="flex flex-wrap justify-end gap-2">
            <Button
              variant="outline"
              disabled={pending}
              onClick={() => setSelected(null)}
            >
              Cancel
            </Button>
            <Button
              variant="outline"
              className="border-negative/40 bg-negative/5 text-negative hover:bg-negative/10"
              disabled={pending}
              onClick={() => {
                void removeSubmission();
              }}
            >
              {pending ? 'Deleting…' : 'Delete submission'}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
