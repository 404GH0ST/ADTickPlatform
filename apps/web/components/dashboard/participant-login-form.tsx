"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusBanner } from "@/components/ui/status-banner";
import { parseApiError } from "@/lib/api-utils";

export function ParticipantLoginForm() {
  const router = useRouter();
  const [mode, setMode] = useState<"login" | "register" | "join">("login");
  const [teamKey, setTeamKey] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);

    try {
      const response = await fetch(
        mode === "join"
          ? "/api/platform/session/join"
          : mode === "register"
            ? "/api/platform/session/register"
            : "/api/platform/session/login",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(
            mode === "join"
              ? {
                  team_key: teamKey,
                  display_name: displayName,
                  email,
                  password,
                }
              : mode === "register"
                ? {
                    display_name: displayName,
                    email,
                    password,
                  }
                : { email, password },
          ),
        },
      );

      if (!response.ok) {
        throw new Error(
          await parseApiError(
            response,
            mode === "join"
              ? "/api/platform/session/join"
              : mode === "register"
                ? "/api/platform/session/register"
                : "/api/platform/session/login",
          ),
        );
      }

      router.push("/services");
      router.refresh();
    } catch (submitError) {
      setError(
        submitError instanceof Error
          ? submitError.message
          : mode === "join"
            ? "team join failed"
            : mode === "register"
              ? "player registration failed"
              : "participant login failed",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <form className="grid gap-4" onSubmit={submit}>
      <div className="grid grid-cols-3 gap-2 rounded-sm border border-border bg-muted p-1">
        <button
          type="button"
          className={`rounded-sm px-3 py-2 text-sm font-semibold transition ${
            mode === "login"
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
          onClick={() => {
            setMode("login");
            setError(null);
          }}
        >
          Sign In
        </button>
        <button
          type="button"
          className={`rounded-sm px-3 py-2 text-sm font-semibold transition ${
            mode === "register"
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
          onClick={() => {
            setMode("register");
            setError(null);
          }}
        >
          Register
        </button>
        <button
          type="button"
          className={`rounded-sm px-3 py-2 text-sm font-semibold transition ${
            mode === "join"
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
          onClick={() => {
            setMode("join");
            setError(null);
          }}
        >
          Join Team
        </button>
      </div>

      {mode === "join" || mode === "register" ? (
        <>
          {mode === "join" ? (
            <div className="grid gap-2">
              <Label htmlFor="participant-team-key">Team key</Label>
              <Input
                id="participant-team-key"
                autoComplete="one-time-code"
                value={teamKey}
                onChange={(event) => setTeamKey(event.target.value)}
                required={mode === "join"}
              />
            </div>
          ) : null}

          <div className="grid gap-2">
            <Label htmlFor="participant-display-name">Display name</Label>
            <Input
              id="participant-display-name"
              autoComplete="name"
              value={displayName}
              onChange={(event) => setDisplayName(event.target.value)}
              required={mode === "join" || mode === "register"}
            />
          </div>
        </>
      ) : null}

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
          {pending
            ? mode === "join"
              ? "Joining..."
              : mode === "register"
                ? "Registering..."
                : "Signing In..."
            : mode === "join"
              ? "Join Team"
              : mode === "register"
                ? "Register"
                : "Sign In"}
        </Button>
      </div>
    </form>
  );
}
