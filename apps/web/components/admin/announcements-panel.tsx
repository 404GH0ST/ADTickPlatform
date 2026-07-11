"use client";

import {
  useCallback,
  useEffect,
  useState,
  type FormEvent,
  type ReactElement,
} from "react";
import { LoaderCircle, Megaphone } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { StatusBanner } from "@/components/ui/status-banner";
import { processApiResponse, parseApiError } from "@/lib/api-utils";
import type { AdminMatchAnnouncement } from "@/lib/admin-dashboard-types";

export function AnnouncementsPanel(): ReactElement {
  const [items, setItems] = useState<AdminMatchAnnouncement[]>([]);
  const [body, setBody] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    const response = await fetch("/api/admin/announcements", {
      cache: "no-store",
    });
    if (!response.ok) {
      throw new Error(await parseApiError(response, "/api/admin/announcements"));
    }
    setItems(
      await processApiResponse<AdminMatchAnnouncement[]>(
        response,
        "/api/admin/announcements",
      ),
    );
  }, []);

  useEffect(() => {
    void refresh().catch((loadError: unknown) => {
      setError(
        loadError instanceof Error
          ? loadError.message
          : "failed to load announcements",
      );
    });
  }, [refresh]);

  async function createAnnouncement(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);
    setNote(null);
    try {
      const response = await fetch("/api/admin/announcements", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body }),
      });
      if (!response.ok) {
        throw new Error(
          await parseApiError(response, "/api/admin/announcements"),
        );
      }
      setBody("");
      setNote("Announcement published to participants.");
      await refresh();
    } catch (createError) {
      setError(
        createError instanceof Error
          ? createError.message
          : "create announcement failed",
      );
    } finally {
      setPending(false);
    }
  }

  async function removeAnnouncement(id: number) {
    setPending(true);
    setError(null);
    try {
      const response = await fetch(`/api/admin/announcements/${id}`, {
        method: "DELETE",
      });
      if (!response.ok) {
        throw new Error(
          await parseApiError(response, `/api/admin/announcements/${id}`),
        );
      }
      await refresh();
    } catch (deleteError) {
      setError(
        deleteError instanceof Error
          ? deleteError.message
          : "delete announcement failed",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <Card data-testid="announcements-panel">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Megaphone className="h-4 w-4 text-primary" />
          <CardTitle className="text-base">Match announcements</CardTitle>
        </div>
        <CardDescription>
          Broadcast short messages to the participant shell during the match.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        <form className="grid gap-2" onSubmit={createAnnouncement}>
          <textarea
            className="min-h-20 w-full rounded-sm border border-input bg-background px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
            value={body}
            onChange={(event) => setBody(event.target.value)}
            placeholder="Warmup starts in 10 minutes. WireGuard must be up."
            maxLength={4000}
          />
          <div>
            <Button type="submit" disabled={pending || body.trim() === ""}>
              {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
              Publish
            </Button>
          </div>
        </form>
        {error ? <StatusBanner message={error} variant="error" /> : null}
        {note ? <StatusBanner message={note} variant="success" /> : null}
        <ul className="grid gap-2">
          {items.map((item) => (
            <li
              key={item.id}
              className="flex items-start justify-between gap-3 rounded-sm border px-3 py-2 text-sm"
            >
              <div>
                <p>{item.body}</p>
                <p className="text-xs text-muted-foreground">{item.created_at}</p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={pending}
                onClick={() => void removeAnnouncement(item.id)}
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
