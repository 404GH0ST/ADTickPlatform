"use client";

import { LoaderCircle } from "lucide-react";

import { Button, Input, Label, StatusBanner, cn, type ParticipantLoginState } from "@/components/theme/kit";

/** Sign in / register with a pill switch. Same auth flow as the stock form. */
export function NbLogin({
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
  const register = mode === "register";
  return (
    <form data-testid="nb-login" className="grid gap-4" onSubmit={onSubmit}>
      <div
        role="group"
        aria-label="Account mode"
        className="grid grid-cols-2 gap-1 rounded-[var(--nb-radius)] border-[3px] border-border bg-muted p-1"
      >
        {(["login", "register"] as const).map((option) => (
          <button
            key={option}
            type="button"
            onClick={() => setMode(option)}
            aria-pressed={mode === option}
            className={cn(
              "nb-display rounded-[var(--nb-radius-sm)] border-2 px-3 py-1.5 text-sm font-bold transition-colors",
              mode === option
                ? "nb-fill-aqua border-[var(--nb-ink)] shadow-[2px_2px_0_var(--nb-shadow)]"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {option === "login" ? "Sign In" : "Register"}
          </button>
        ))}
      </div>
      {register ? (
        <div className="grid gap-1.5">
          <Label htmlFor="participant-display-name" className="font-bold">
            Display name
          </Label>
          <Input
            id="participant-display-name"
            autoComplete="name"
            value={displayName}
            onChange={(event) => setDisplayName(event.target.value)}
            required
          />
        </div>
      ) : null}
      <div className="grid gap-1.5">
        <Label htmlFor="participant-email" className="font-bold">
          Email
        </Label>
        <Input
          id="participant-email"
          type="email"
          autoComplete="username"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="participant-password" className="font-bold">
          Password
        </Label>
        <Input
          id="participant-password"
          type="password"
          autoComplete={register ? "new-password" : "current-password"}
          minLength={register ? minPasswordLength : undefined}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          required
        />
        {register ? (
          <p className="text-xs text-muted-foreground">At least {minPasswordLength} characters.</p>
        ) : null}
      </div>
      {error ? <StatusBanner message={error} variant="error" /> : null}
      <Button type="submit" size="lg" className="mt-1 h-12 w-full text-base" disabled={pending}>
        {pending ? <LoaderCircle className="size-4 animate-spin" aria-hidden /> : null}
        {pending ? (register ? "Registering…" : "Signing in…") : register ? "Register" : "Sign In"}
      </Button>
    </form>
  );
}
