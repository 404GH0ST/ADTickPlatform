"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { ChevronDown, Menu } from "lucide-react";

/**
 * Header keys folded into one "Menu" key so the title banner keeps its
 * width. A <details> disclosure: works without JS, closes on Escape, on an
 * outside click, and after an item inside is chosen.
 */
export function NbMenu({ children, label = "Menu" }: { children: ReactNode; label?: string }) {
  const ref = useRef<HTMLDetailsElement>(null);

  useEffect(() => {
    const menu = ref.current;
    if (!menu) return;
    const close = (refocus: boolean) => {
      if (!menu.open) return;
      menu.open = false;
      if (refocus) menu.querySelector("summary")?.focus();
    };
    const onPointer = (event: PointerEvent) => {
      if (!menu.contains(event.target as Node)) close(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && menu.contains(document.activeElement)) close(true);
    };
    // A null target means focus left the page or landed on nothing, so the
    // panel stays put; a real target outside it means the user tabbed away.
    const onFocusOut = (event: FocusEvent) => {
      if (event.relatedTarget instanceof Node && !menu.contains(event.relatedTarget)) close(false);
    };
    document.addEventListener("pointerdown", onPointer);
    document.addEventListener("keydown", onKey);
    menu.addEventListener("focusout", onFocusOut);
    return () => {
      document.removeEventListener("pointerdown", onPointer);
      document.removeEventListener("keydown", onKey);
      menu.removeEventListener("focusout", onFocusOut);
    };
  }, []);

  return (
    <details ref={ref} className="nb-menu relative" data-testid="nb-account-menu">
      <summary className="nb-menu-key">
        <Menu className="size-4" strokeWidth={2.4} aria-hidden />
        {label}
        <ChevronDown className="nb-menu-chevron size-3.5" strokeWidth={2.6} aria-hidden />
      </summary>
      <div
        className="nb-menu-panel"
        onClick={(event) => {
          // Portal events bubble through React, but the dialog is outside this panel.
          const item = event.target instanceof Element ? event.target.closest("a, button") : null;
          const menu = ref.current;
          if (!item || !menu || !event.currentTarget.contains(item)) return;
          menu.open = false;
          menu.querySelector("summary")?.focus();
        }}
      >
        {children}
      </div>
    </details>
  );
}
