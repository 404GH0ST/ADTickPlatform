"use client";

import type { ReactElement } from "react";
import { LoaderCircle, Megaphone } from "lucide-react";

import { type AnnouncementsState, useAnnouncements } from "@/components/admin/use-announcements";
import { ThemeSlot } from "@/components/theme/theme-slot";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { StatusBanner } from "@/components/ui/status-banner";
import { formatIndonesianDate } from "@/lib/date-format";

/** Organizer match announcements. Logic in useAnnouncements; themeable via OrganizerAnnouncements. */
export function AnnouncementsPanel(): ReactElement {
  return <ThemeSlot name="OrganizerAnnouncements" fallback={DefaultAnnouncementsView} props={useAnnouncements()} />;
}

export function DefaultAnnouncementsView({
  items,
  body,
  setBody,
  pending,
  error,
  note,
  maxLength,
  onPublish,
  onDelete,
}: AnnouncementsState): ReactElement {
  return (
    <Card data-testid="announcements-panel">
      <CardHeader>
        <div className="flex items-start gap-2">
          <Megaphone className="mt-1 h-4 w-4 shrink-0 text-primary-ink" />
          <CardTitle className="min-w-0 wrap-anywhere text-base">Match announcements</CardTitle>
        </div>
        <CardDescription>
          Broadcast short messages to the participant shell during the match.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <form className="flex flex-col gap-2 sm:flex-row sm:items-start" onSubmit={onPublish}>
          <label htmlFor="announcement-body" className="sr-only">
            Announcement
          </label>
          <textarea
            id="announcement-body"
            rows={2}
            className="min-h-11 w-full flex-1 resize-y rounded-sm border border-input bg-background px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
            value={body}
            onChange={(event) => setBody(event.target.value)}
            placeholder="Warmup starts in 10 minutes. WireGuard must be up."
            maxLength={maxLength}
          />
          <Button type="submit" className="shrink-0" disabled={pending || body.trim() === ""} holdFocus>
            {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
            Publish
          </Button>
        </form>
        {error ? <StatusBanner message={error} variant="error" /> : null}
        {note ? <StatusBanner message={note} variant="success" /> : null}
        <ul className="grid gap-2">
          {items.map((item) => (
            <li
              key={item.id}
              data-slot="well"
              className="flex flex-wrap items-start justify-between gap-3 rounded-sm border px-3 py-2 text-sm"
            >
              <div className="min-w-0 wrap-anywhere">
                <p>{item.body}</p>
                <time dateTime={item.created_at} className="block text-xs text-muted-foreground">
                  {formatIndonesianDate(item.created_at)}
                </time>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={pending}
                holdFocus
                onClick={() => onDelete(item.id)}
              >
                Delete
              </Button>
            </li>
          ))}
          {items.length === 0 ? (
            <li className="text-sm text-muted-foreground">
              No announcements yet.
            </li>
          ) : null}
        </ul>
      </CardContent>
    </Card>
  );
}
