"use client";

import type { FormEvent, ReactNode } from "react";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Settings } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { StatusBanner } from "@/components/ui/status-banner";
import type { PlatformOverview } from "@/lib/dashboard-types";
import { parseApiError } from "@/lib/api-utils";

type Draft = {
  displayName: string;
  email: string;
  teamName: string;
  teamContactEmail: string;
};

export function ParticipantAccountSettings({
  overview,
}: {
  overview: PlatformOverview;
}) {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft>(() => draftFromOverview(overview));
  const hasTeam = (overview.teamID ?? 0) > 0 && overview.role !== "organizer";

  useEffect(() => {
    if (open) {
      setDraft(draftFromOverview(overview));
      setMessage(null);
      setError(null);
    }
  }, [open, overview]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setMessage(null);
    setError(null);

    try {
      const response = await fetch("/api/platform/session/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          display_name: draft.displayName,
          email: draft.email,
          team_name: draft.teamName,
          team_contact_email: draft.teamContactEmail,
        }),
      });
      if (!response.ok) {
        throw new Error(
          await parseApiError(response, "/api/platform/session/profile"),
        );
      }
      setMessage("Account settings saved.");
      router.refresh();
    } catch (updateError) {
      setError(
        updateError instanceof Error
          ? updateError.message
          : "profile update failed",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <>
      <Button
        aria-label="Account settings"
        variant="outline"
        onClick={() => setOpen(true)}
      >
        <Settings className="h-4 w-4" />
        Account
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Account settings</DialogTitle>
            <DialogDescription>
              Update your player profile and current team information.
            </DialogDescription>
          </DialogHeader>
          <form className="space-y-4" onSubmit={submit}>
            <section className="grid gap-3">
              <div>
                <p className="text-sm font-medium text-foreground">
                  Player information
                </p>
                <p className="text-sm text-muted-foreground">
                  Used for sign-in, audit entries, and WireGuard ownership.
                </p>
              </div>
              <Field label="Display name" htmlFor="participant-display-name">
                <Input
                  id="participant-display-name"
                  autoComplete="name"
                  value={draft.displayName}
                  onChange={(event) =>
                    setDraft({ ...draft, displayName: event.target.value })
                  }
                  required
                />
              </Field>
              <Field label="Email" htmlFor="participant-email">
                <Input
                  id="participant-email"
                  type="email"
                  autoComplete="email"
                  value={draft.email}
                  onChange={(event) =>
                    setDraft({ ...draft, email: event.target.value })
                  }
                  required
                />
              </Field>
            </section>

            {hasTeam ? (
              <section className="grid gap-3 border-t pt-4">
                <div>
                  <p className="text-sm font-medium text-foreground">
                    Team information
                  </p>
                  <p className="text-sm text-muted-foreground">
                    The team name is reflected in scoreboard and attack views.
                  </p>
                </div>
                <Field label="Team name" htmlFor="participant-team-name">
                  <Input
                    id="participant-team-name"
                    value={draft.teamName}
                    onChange={(event) =>
                      setDraft({ ...draft, teamName: event.target.value })
                    }
                    required
                  />
                </Field>
                <Field label="Team email" htmlFor="participant-team-email">
                  <Input
                    id="participant-team-email"
                    type="email"
                    value={draft.teamContactEmail}
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        teamContactEmail: event.target.value,
                      })
                    }
                    required
                  />
                </Field>
              </section>
            ) : (
              <StatusBanner
                message="Join a team before editing team information."
                variant="warning"
              />
            )}

            {error ? <StatusBanner message={error} variant="error" /> : null}
            {message ? (
              <StatusBanner message={message} variant="success" />
            ) : null}

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setOpen(false)}
              >
                Close
              </Button>
              <Button type="submit" disabled={pending}>
                {pending ? "Saving..." : "Save"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}

function draftFromOverview(overview: PlatformOverview): Draft {
  return {
    displayName: overview.displayName ?? "",
    email: overview.email ?? "",
    teamName: overview.teamName ?? "",
    teamContactEmail: overview.teamContactEmail ?? "",
  };
}

function Field({
  children,
  htmlFor,
  label,
}: {
  children: ReactNode;
  htmlFor: string;
  label: string;
}) {
  return (
    <label className="grid gap-1.5 text-sm font-medium" htmlFor={htmlFor}>
      {label}
      {children}
    </label>
  );
}
