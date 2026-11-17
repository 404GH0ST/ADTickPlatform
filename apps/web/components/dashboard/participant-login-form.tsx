"use client";

import { ThemeSlot } from "@/components/theme/theme-slot";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusBanner } from "@/components/ui/status-banner";
import {
  type ParticipantLoginState,
  useParticipantLogin,
} from "@/components/dashboard/use-participant-login";

/** Login / register form. Logic lives in useParticipantLogin; themes can replace the view via the Login slot. */
export function ParticipantLoginForm() {
  const state = useParticipantLogin();
  return <ThemeSlot name="Login" fallback={DefaultLoginView} props={state} />;
}

export function DefaultLoginView({
  mode,
  setMode,
  displayName,
  setDisplayName,
  email,
  setEmail,
  password,
  setPassword,
  pending,
  error,
  minPasswordLength,
  onSubmit,
}: ParticipantLoginState) {
  return (
    <form className="grid gap-4" onSubmit={onSubmit}>
      <div className="grid grid-cols-2 gap-2 rounded-sm border border-border bg-muted p-1">
        <button
          type="button"
          className={`rounded-sm px-3 py-2 text-sm font-semibold transition ${
            mode === "login"
              ? "bg-card text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
          onClick={() => setMode("login")}
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
          onClick={() => setMode("register")}
        >
          Register
        </button>
      </div>

      {mode === "register" ? (
        <div className="grid gap-2">
          <Label htmlFor="participant-display-name">Display name</Label>
          <Input
            id="participant-display-name"
            autoComplete="name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            required={mode === "register"}
          />
        </div>
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
          autoComplete={mode === "register" ? "new-password" : "current-password"}
          minLength={mode === "register" ? minPasswordLength : undefined}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          required
        />
      </div>

      {error ? <StatusBanner message={error} variant="error" /> : null}

      <div className="flex items-center gap-3">
        <Button type="submit" className="w-full" disabled={pending}>
          {pending
            ? mode === "register"
              ? "Registering..."
              : "Signing In..."
            : mode === "register"
              ? "Register"
              : "Sign In"}
        </Button>
      </div>
    </form>
  );
}
