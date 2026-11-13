import Link from 'next/link';
import { OrganizerShell } from '@/components/admin/organizer-shell';
import {
  RefreshSubmissions,
  SubmissionsTable,
} from '@/components/admin/submissions-table';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { loadAdminDashboardData } from '@/lib/admin-dashboard-data';
import { listAdminSubmissions } from '@/lib/admin-api';
import { parseAttackFeedQuery } from '@/lib/route-query';
import { buildAttackFilterOptions } from '@/lib/dashboard-utils';

export const dynamic = 'force-dynamic';
type SearchParams = Record<string, string | string[] | undefined>;

export default async function AdminSubmissionsPage({
  searchParams,
}: {
  searchParams: Promise<SearchParams>;
}) {
  const raw = await searchParams;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(raw)) {
    const first = Array.isArray(value) ? value[0] : value;
    if (first) params.set(key, first);
  }
  const query = {
    ...parseAttackFeedQuery(params, 25),
    status: params.get('status')?.trim() || undefined,
    flag: params.get('flag')?.trim() || undefined,
  };
  const [dashboard, result] = await Promise.all([
    loadAdminDashboardData(),
    listAdminSubmissions(query)
      .then((page) => ({ page, error: undefined as string | undefined }))
      .catch(() => ({
        page: {
          items: [],
          limit: query.limit ?? 25,
          offset: query.offset ?? 0,
          total_count: 0,
          has_prev: false,
          has_next: false,
        },
        error: 'Could not load submission history. Refresh to try again.',
      })),
  ]);
  const page = result.page;
  function href(offset: number) {
    const next = new URLSearchParams(params);
    next.set('limit', String(page.limit));
    next.set('offset', String(offset));
    return `/admin/submissions?${next}`;
  }
  const fields = [
    {
      name: 'attacker',
      label: 'Submitting team',
      options: buildAttackFilterOptions(
        dashboard.teams.map((team) => team.name),
        query.attacker,
      ),
    },
    {
      name: 'victim',
      label: 'Victim team',
      options: buildAttackFilterOptions(
        dashboard.teams.map((team) => team.name),
        query.victim,
      ),
    },
    {
      name: 'service',
      label: 'Challenge',
      options: buildAttackFilterOptions(
        dashboard.challenges.map((challenge) => challenge.name),
        query.service,
      ),
    },
  ];
  return (
    <OrganizerShell
      activePath="/admin/submissions"
      title="Submissions"
      overview={dashboard.overview}
    >
      <section
        className="min-w-0 rounded-sm border bg-card"
        aria-labelledby="submission-history-title"
      >
        <div className="flex flex-wrap items-center justify-between gap-3 border-b p-4">
          <div>
            <h2
              id="submission-history-title"
              className="text-base font-semibold"
            >
              Submission history
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Accepted, rejected, and duplicate flag attempts. Deleting an
              accepted capture updates scoring.
            </p>
          </div>
          <RefreshSubmissions />
        </div>
        <form className="space-y-3 border-b p-4" action="/admin/submissions">
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {fields.map((field) => (
              <div key={field.name} className="space-y-2">
                <Label htmlFor={`submission-${field.name}`}>
                  {field.label}
                </Label>
                <select
                  id={`submission-${field.name}`}
                  name={field.name}
                  defaultValue={params.get(field.name) ?? ''}
                  className="h-11 w-full rounded-sm border border-input bg-card px-3 text-sm sm:h-9"
                >
                  <option value="">All</option>
                  {field.options.map((option) => (
                    <option key={option}>{option}</option>
                  ))}
                </select>
              </div>
            ))}
            <div className="space-y-2">
              <Label htmlFor="submission-status">Verdict</Label>
              <select
                id="submission-status"
                name="status"
                defaultValue={query.status ?? ''}
                className="h-11 w-full rounded-sm border border-input bg-card px-3 text-sm sm:h-9"
              >
                <option value="">All verdicts</option>
                <option value="accepted">Accepted</option>
                <option value="invalid">Rejected</option>
                <option value="duplicate">Duplicate</option>
              </select>
            </div>
          </div>
          <details
            open={Boolean(query.flag || query.tick_from || query.tick_to)}
          >
            <summary className="w-fit cursor-pointer text-sm text-muted-foreground focus-visible:outline-2 focus-visible:outline-ring">
              Flag, ticks, and page size
            </summary>
            <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-[2fr_1fr_1fr_1fr]">
              <div className="space-y-2">
                <Label htmlFor="submission-flag">Flag contains</Label>
                <Input
                  id="submission-flag"
                  name="flag"
                  defaultValue={query.flag}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="submission-tick-from">From tick</Label>
                <Input
                  id="submission-tick-from"
                  name="tick_from"
                  type="number"
                  min="0"
                  defaultValue={query.tick_from}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="submission-tick-to">Through tick</Label>
                <Input
                  id="submission-tick-to"
                  name="tick_to"
                  type="number"
                  min="0"
                  defaultValue={query.tick_to}
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="submission-limit">Rows per page</Label>
                <select
                  id="submission-limit"
                  name="limit"
                  defaultValue={page.limit}
                  className="h-9 w-full rounded-sm border border-input bg-card px-3 text-sm"
                >
                  {[25, 50, 100, 200].map((limit) => (
                    <option key={limit}>{limit}</option>
                  ))}
                </select>
              </div>
            </div>
          </details>
          <div className="flex flex-wrap gap-2">
            <Button type="submit">Apply filters</Button>
            <Button asChild variant="outline">
              <Link href="/admin/submissions">Reset</Link>
            </Button>
          </div>
        </form>
        <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3 text-sm text-muted-foreground">
          <span>
            {page.total_count === 0
              ? '0 submissions'
              : `Showing ${page.offset + 1}–${Math.min(page.offset + page.items.length, page.total_count)} of ${page.total_count}`}
          </span>
          <span>First blood is awarded once per challenge.</span>
        </div>
        <SubmissionsTable page={page} error={result.error} />
        <div className="flex justify-end gap-2 border-t p-3">
          {page.has_prev ? (
            <Button asChild variant="outline">
              <Link href={href(Math.max(0, page.offset - page.limit))}>
                Previous
              </Link>
            </Button>
          ) : (
            <Button disabled variant="outline">
              Previous
            </Button>
          )}
          {page.has_next ? (
            <Button asChild variant="outline">
              <Link href={href(page.offset + page.limit)}>Next</Link>
            </Button>
          ) : (
            <Button disabled variant="outline">
              Next
            </Button>
          )}
        </div>
      </section>
    </OrganizerShell>
  );
}
