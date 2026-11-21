"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";

import {
  browserRequestErrorMessage,
  parseApiError,
  processApiResponse,
} from "@/lib/api-utils";
import type { AdminMatchAnnouncement } from "@/lib/admin-dashboard-types";

/** State and actions for organizer match announcements (OrganizerAnnouncements slot). */
export type AnnouncementsState = {
  items: AdminMatchAnnouncement[];
  body: string;
  setBody: (value: string) => void;
  pending: boolean;
  error: string | null;
  note: string | null;
  /** Max announcement length accepted by the API. */
  maxLength: number;
  /** Use as a <form onSubmit>: publishes `body`. */
  onPublish: (event?: FormEvent<HTMLFormElement>) => void;
  onDelete: (id: number) => void;
};

export function useAnnouncements(): AnnouncementsState {
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
        browserRequestErrorMessage(loadError, "failed to load announcements"),
      );
    });
  }, [refresh]);

  async function createAnnouncement(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
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
        browserRequestErrorMessage(createError, "create announcement failed"),
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
        browserRequestErrorMessage(deleteError, "delete announcement failed"),
      );
    } finally {
      setPending(false);
    }
  }

  return {
    items,
    body,
    setBody,
    pending,
    error,
    note,
    maxLength: 4000,
    onPublish: (event) => void createAnnouncement(event),
    onDelete: (id) => void removeAnnouncement(id),
  };
}
