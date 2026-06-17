"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { StatusBanner } from "@/components/ui/status-banner";
import { parseApiError } from "@/lib/api-utils";

export function ParticipantJoinTeamForm() {
  const router = useRouter();
  const [teamKey, setTeamKey] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);

    try {
      const response = await fetch("/api/platform/session/team", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ team_key: teamKey }),
      });

      if (!response.ok) {
        throw new Error(
          await parseApiError(response, "/api/platform/session/team"),
        );
      }

      router.refresh();
    } catch (joinError) {
      setError(joinError instanceof Error ? joinError.message : "team join failed");
    } finally {
      setPending(false);
    }
  }

  return (
    <form
      className="flex flex-col gap-3 rounded-sm border border-border bg-card p-4 sm:flex-row sm:items-start"
      onSubmit={submit}
    >
      <div className="grid flex-1 gap-2">
        <label htmlFor="participant-current-team-key" className="text-sm font-medium">
          Team key
        </label>
        <Input
          id="participant-current-team-key"
          autoComplete="one-time-code"
          value={teamKey}
          onChange={(event) => setTeamKey(event.target.value)}
          required
        />
        {error ? <StatusBanner message={error} variant="error" /> : null}
      </div>
      <Button type="submit" disabled={pending} className="sm:mt-7">
        {pending ? "Joining..." : "Join Team"}
      </Button>
    </form>
  );
}
