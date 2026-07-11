"use client";

import { useEffect, useState, type ReactElement } from "react";

import { StatusBanner } from "@/components/ui/status-banner";
import { processApiResponse } from "@/lib/api-utils";

type Announcement = {
  id: number;
  body: string;
  created_at: string;
};

export function AnnouncementBanner(): ReactElement | null {
  const [items, setItems] = useState<Announcement[]>([]);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const response = await fetch("/api/platform/announcements", {
          cache: "no-store",
        });
        if (!response.ok) {
          return;
        }
        const payload = await processApiResponse<Announcement[]>(
          response,
          "/api/platform/announcements",
        );
        if (!cancelled) {
          setItems(payload.slice(0, 5));
        }
      } catch {
        // Keep quiet if announcements are unavailable.
      }
    }
    void load();
    const timer = window.setInterval(() => {
      void load();
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
