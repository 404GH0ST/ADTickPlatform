'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';

import { Button } from '@/components/ui/button';

/**
 * For a dialog with a form. Once a field has changed, dismissing the dialog
 * asks first. Put `markChanged` on the dialog content's onChangeCapture, route
 * dismissals through `requestClose`, and show `prompt` in place of the footer.
 * While `busy`, a save can't be taken back, so the prompt says so instead of
 * offering to discard it.
 */
export function useUnsavedGuard({
  open,
  onClose,
  enabled = true,
  busy = false,
}: {
  open: boolean;
  onClose: () => void;
  enabled?: boolean;
  busy?: boolean;
}): {
  markChanged: () => void;
  markSaved: () => void;
  prompt: ReactNode;
  requestClose: () => void;
} {
  const changed = useRef(false);
  const focusBeforePrompt = useRef<HTMLElement | null>(null);
  const keepEditingButton = useRef<HTMLButtonElement>(null);
  const [asking, setAsking] = useState(false);
  const saving = enabled && busy;
  const wasSaving = useRef(false);

  useEffect(() => {
    changed.current = false;
    setAsking(false);
  }, [open]);

  useEffect(() => {
    if (asking) keepEditingButton.current?.focus();
  }, [asking]);

  function keepEditing() {
    setAsking(false);
    const field = focusBeforePrompt.current;
    if (field?.isConnected) requestAnimationFrame(() => field.focus());
  }

  // The question no longer applies once the save has finished.
  useEffect(() => {
    if (wasSaving.current && !saving && asking) {
      setAsking(false);
      const field = focusBeforePrompt.current;
      if (field?.isConnected) requestAnimationFrame(() => field.focus());
    }
    wasSaving.current = saving;
  }, [asking, saving]);

  function ask() {
    focusBeforePrompt.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setAsking(true);
  }

  function requestClose() {
    if (asking) {
      keepEditing();
    } else if (saving || (enabled && changed.current)) {
      ask();
    } else {
      onClose();
    }
  }

  return {
    markChanged: () => {
      changed.current = true;
    },
    markSaved: () => {
      changed.current = false;
    },
    requestClose,
    prompt: asking ? (
      <div role="alert" className="flex w-full flex-wrap items-center justify-between gap-3">
        <p className="text-sm font-medium text-foreground">
          {saving ? "Still saving. Closing won't cancel it." : 'Discard your changes?'}
        </p>
        <div className="flex flex-wrap gap-2">
          <Button ref={keepEditingButton} type="button" variant="outline" onClick={keepEditing}>
            {saving ? 'Keep waiting' : 'Keep editing'}
          </Button>
          <Button type="button" className={saving ? undefined : 'button-danger-solid'} onClick={onClose}>
            {saving ? 'Close anyway' : 'Discard changes'}
          </Button>
        </div>
      </div>
    ) : null,
  };
}
