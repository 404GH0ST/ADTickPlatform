"use client";

import { useCallback, useEffect, useState } from "react";
import type { ReactElement } from "react";

import type { AdminPlatformSettings } from "@/lib/admin-dashboard-types";
import {
  getAdminPlatformSettings,
  reloadAdminFlagFormat,
  updateAdminPlatformSettings,
} from "@/lib/admin-api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";

type SaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "reloading" }
  | { kind: "saved" }
  | { kind: "error"; message: string };

const flagFormatPattern = /^[A-Za-z][A-Za-z0-9_-]{0,31}$/;

export function PlatformSettingsCard(): ReactElement {
  const [settings, setSettings] = useState<AdminPlatformSettings | null>(null);
  const [prefix, setPrefix] = useState<string>("");
  const [saveState, setSaveState] = useState<SaveState>({ kind: "idle" });

  const load = useCallback(async () => {
    try {
      const data = await getAdminPlatformSettings();
      setSettings(data);
      setPrefix(data.flag_format_prefix);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "failed to load platform settings.";
      setSaveState({ kind: "error", message });
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const trimmed = prefix.trim();
  const isValid = flagFormatPattern.test(trimmed);
  const inSync = settings
    ? settings.flag_format_prefix === settings.flag_format_active
    : true;
  const dirty = settings ? trimmed !== settings.flag_format_prefix : false;

  const onSave = useCallback(async () => {
    if (!isValid) {
      setSaveState({
        kind: "error",
        message: "flag_format_prefix must start with a letter and contain only letters, digits, '-' or '_' (max 32 chars).",
      });
      return;
    }
    setSaveState({ kind: "saving" });
    try {
      const updated = await updateAdminPlatformSettings({ flag_format_prefix: trimmed });
      setSettings(updated);
      setSaveState({ kind: "saved" });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "failed to save platform settings.";
      setSaveState({ kind: "error", message });
    }
  }, [isValid, trimmed]);

  const onSaveAndReload = useCallback(async () => {
    if (!isValid) {
      setSaveState({
        kind: "error",
        message: "flag_format_prefix must start with a letter and contain only letters, digits, '-' or '_' (max 32 chars).",
      });
      return;
    }
    setSaveState({ kind: "saving" });
    try {
      const updated = await updateAdminPlatformSettings({ flag_format_prefix: trimmed });
      setSettings(updated);
      setSaveState({ kind: "reloading" });
      const reloaded = await reloadAdminFlagFormat();
      setSettings(reloaded);
      setSaveState({ kind: "saved" });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "failed to save or reload flag format.";
      setSaveState({ kind: "error", message });
    }
  }, [isValid, trimmed]);

  const onReloadOnly = useCallback(async () => {
    setSaveState({ kind: "reloading" });
    try {
      const reloaded = await reloadAdminFlagFormat();
      setSettings(reloaded);
      setSaveState({ kind: "saved" });
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "flag format reload failed.";
      setSaveState({ kind: "error", message });
    }
  }, []);

  return (
    <Card>
      <CardHeader className="space-y-1.5 pb-4">
        <CardTitle>Flag Format</CardTitle>
        <CardDescription>
          Configure the flag prefix issued by the checker and accepted on submission. The platform
          signs flags with HMAC-SHA256; the prefix and braces wrap the signed payload. Default
          is <code className="font-mono text-xs">PLAYIT{'{...}'}</code>.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
          <div className="flex-1 space-y-1.5">
            <label
              htmlFor="platform-flag-format-prefix"
              className="text-xs font-medium text-muted-foreground"
            >
              Flag Format Prefix
            </label>
            <Input
              id="platform-flag-format-prefix"
              name="flag_format_prefix"
              data-testid="input-flag-format-prefix"
              value={prefix}
              onChange={(event) => setPrefix(event.target.value)}
              placeholder="PLAYIT"
              spellCheck={false}
              autoComplete="off"
              disabled={saveState.kind === "saving" || saveState.kind === "reloading"}
            />
            <p className="text-xs text-muted-foreground">
              Example flag: <code className="font-mono">{trimmed || "PLAYIT"}{'{payload.signature}'}</code>
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              data-testid="button-save-platform-settings"
              onClick={() => {
                void onSave();
              }}
              disabled={
                !isValid ||
                !dirty ||
                saveState.kind === "saving" ||
                saveState.kind === "reloading"
              }
            >
              Save
            </Button>
            <Button
              type="button"
              data-testid="button-save-and-reload-flag-format"
              onClick={() => {
                void onSaveAndReload();
              }}
              disabled={
                !isValid ||
                !dirty ||
                saveState.kind === "saving" ||
                saveState.kind === "reloading"
              }
            >
              {saveState.kind === "saving"
                ? "Saving…"
                : saveState.kind === "reloading"
                  ? "Reloading…"
                  : "Save & Reload"}
            </Button>
            <Button
              type="button"
              variant="ghost"
              data-testid="button-reload-flag-format"
              onClick={() => {
                void onReloadOnly();
              }}
              disabled={saveState.kind === "saving" || saveState.kind === "reloading"}
            >
              Reload Only
            </Button>
          </div>
        </div>

        <div className="grid gap-2 sm:grid-cols-3">
          <div className="rounded border border-border bg-card/40 p-3">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Stored</p>
            <p className="mt-1 font-mono text-sm">
              {settings?.flag_format_prefix ?? "—"}
            </p>
          </div>
          <div className="rounded border border-border bg-card/40 p-3">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Active in game-core</p>
            <p className="mt-1 font-mono text-sm">
              {settings?.flag_format_active ?? "—"}
            </p>
          </div>
          <div className="rounded border border-border bg-card/40 p-3">
            <p className="text-xs uppercase tracking-wide text-muted-foreground">Sync</p>
            <p className="mt-1 flex items-center gap-2 text-sm">
              {inSync ? (
                <Badge className="tone-success" variant="outline" data-testid="badge-flag-format-sync">
                  in sync
                </Badge>
              ) : (
                <Badge className="tone-warning" variant="outline" data-testid="badge-flag-format-sync">
                  out of sync
                </Badge>
              )}
            </p>
          </div>
        </div>

        {settings?.updated_at ? (
          <p className="text-xs text-muted-foreground">
            Last updated: {settings.updated_at}
            {settings.updated_by ? ` by ${settings.updated_by}` : ""}
          </p>
        ) : null}

        {saveState.kind === "error" ? (
          <p
            className="text-sm text-destructive"
            role="alert"
            data-testid="error-platform-settings"
          >
            {saveState.message}
          </p>
        ) : null}

        {saveState.kind === "saved" ? (
          <p
            className="text-sm text-emerald-600"
            data-testid="note-platform-settings"
          >
            Platform settings saved. Flags issued by the checker now use the new prefix.
          </p>
        ) : null}

        {!inSync ? (
          <p className="text-xs text-muted-foreground">
            The stored prefix and the active game-core format differ. Click <strong>Reload Only</strong>{" "}
            to push the stored prefix into game-core, or <strong>Save &amp; Reload</strong> to update and
            reload in one step. Do not change the prefix while a match is running — in-flight flags
            will be rejected as invalid.
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
