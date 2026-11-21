"use client";

import { ThemeSlot } from "@/components/theme/theme-slot";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusBanner } from "@/components/ui/status-banner";
import {
  type ParticipantLoginState,
  useParticipantLogin,
} from "@/components/dashboard/use-participant-login";

/**
 * Login / register card; the heading follows the mode. Logic lives in
 * useParticipantLogin; themes can replace the form via the Login slot.
 */
export function ParticipantLoginCard() {
  const state = useParticipantLogin();
  const [title, otherTitle] = state.mode === "register" ? ["Register", "Sign In"] : ["Sign In", "Register"];
  return (
    <Card className="w-full border-border bg-background shadow-none">
      <CardHeader>
        {/* The unused title holds its height: at large text sizes only one of them wraps. */}
        <CardTitle
          as="h1"
          data-reserve={otherTitle}
          className="grid after:invisible after:col-start-1 after:row-start-1 after:content-[attr(data-reserve)]"
        >
          <span className="col-start-1 row-start-1">{title}</span>
        </CardTitle>
        <CardDescription>
          AD Platform team console. Use your team member account, or
          register with your team&apos;s join key.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <ThemeSlot name="Login" fallback={DefaultLoginView} props={state} />
      </CardContent>
    </Card>
  );
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
  fieldRefs,
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
            ref={fieldRefs.displayName}
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
          ref={fieldRefs.email}
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
          ref={fieldRefs.password}
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
        <Button type="submit" className="w-full" disabled={pending} holdFocus>
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
