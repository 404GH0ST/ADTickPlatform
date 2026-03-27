"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusBanner } from "@/components/ui/status-banner";

type LoginEnvelope =
  | {
      status: "success";
      data: {
        team_id?: number;
        team_name?: string;
        display_name?: string;
      };
    }
  | {
      status: "failed" | "forbidden" | "too many request";
      message: string;
    };

export function ParticipantLoginForm() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);

    try {
      const response = await fetch("/api/platform/session/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ email, password }),
      });

      const payload = (await response.json()) as LoginEnvelope;
      if (!response.ok || payload.status !== "success") {
        throw new Error(
          "message" in payload ? payload.message : "participant login failed",
        );
      }

      router.push("/services");
      router.refresh();
    } catch (submitError) {
      setError(
        submitError instanceof Error
          ? submitError.message
          : "participant login failed",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="grid gap-4" onSubmit={submit}>
      <div className="grid gap-2">
        <Label htmlFor="participant-email">Email</Label>
        <Input
          id="participant-email"
          type="email"
          autoComplete="username"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="participant-password">Password</Label>
        <Input
          id="participant-password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          required
        />
      </div>

      {error ? <StatusBanner message={error} variant="error" /> : null}

      <div className="flex items-center gap-3">
        <Button type="submit" className="w-full" disabled={pending}>
          {pending ? "Signing In..." : "Sign In"}
        </Button>
      </div>
    </form>
  );
}
