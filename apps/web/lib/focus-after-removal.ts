/**
 * Keeps keyboard focus in place when the row holding `control` is removed: it
 * moves to the row that took its place, or the page heading when none is left.
 * The row itself takes focus rather than one of its keys, so a repeated Enter
 * cannot fire that row's Deploy or Delete. Returns a function that stops watching.
 */
export function focusNeighborWhenRemoved(control: Element | null): () => void {
  if (!control) return () => {};
  const row = control.closest("tr, li");
  const neighbors = [row?.nextElementSibling, row?.previousElementSibling];
  const observer = new MutationObserver(() => {
    if (control.isConnected) return;
    observer.disconnect();
    if (document.activeElement && document.activeElement !== document.body) return;
    const target =
      neighbors.find((element) => element?.isConnected && element.getClientRects().length > 0) ??
      document.querySelector("main h1, header h1");
    if (target instanceof HTMLElement) focusWithoutTabStop(target);
  });
  observer.observe(document.body, { childList: true, subtree: true });
  return () => observer.disconnect();
}

function focusWithoutTabStop(element: HTMLElement): void {
  if (!element.hasAttribute("tabindex")) {
    element.tabIndex = -1;
    element.addEventListener("blur", () => element.removeAttribute("tabindex"), { once: true });
  }
  element.focus();
}
