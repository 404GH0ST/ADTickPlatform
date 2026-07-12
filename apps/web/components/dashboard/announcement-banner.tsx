"use client";

import { useEffect, useState, type ReactElement } from "react";

import { StatusBanner } from "@/components/ui/status-banner";
import { processApiResponse } from "@/lib/api-utils";

type Announcement = {
  id: number;
  body: string;
  created_at: string;
};

let cachedAnnouncements: Announcement[] = [];
let announcementRequest: Promise<Announcement[] | null> | null = null;

async function loadAnnouncements(): Promise<Announcement[] | null> {
  if (announcementRequest) {
    return announcementRequest;
  }

  announcementRequest = (async () => {
    try {
      const response = await fetch("/api/platform/announcements", {
        cache: "no-store",
      });
      if (!response.ok) {
        return null;
      }
      const payload = await processApiResponse<Announcement[]>(
        response,
        "/api/platform/announcements",
      );
      cachedAnnouncements = payload.slice(0, 5);
      return cachedAnnouncements;
    } catch {
      // Preserve the last good snapshot if announcements are unavailable.
      return null;
    } finally {
      announcementRequest = null;
    }
  })();

  return announcementRequest;
}

export function AnnouncementBanner(): ReactElement | null {
  const [items, setItems] = useState<Announcement[]>(cachedAnnouncements);

  useEffect(() => {
    let cancelled = false;
    async function refresh() {
      const nextItems = await loadAnnouncements();
      if (!cancelled && nextItems) {
        setItems(nextItems);
      }
    }
    void refresh();
    const timer = window.setInterval(() => {
      void refresh();
    }, 60000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);

  if (items.length === 0) {
    return null;
  }

  return (
    <div className="grid gap-2" data-testid="announcement-banner">
      {items.map((item) => (
        <StatusBanner
          key={item.id}
          message={`Organizer: ${item.body}`}
          variant="info"
        />
      ))}
    </div>
  );
}
