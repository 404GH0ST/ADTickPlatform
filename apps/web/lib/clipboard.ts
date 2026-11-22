/**
 * Copy text, falling back to a hidden textarea and execCommand where the async
 * clipboard API is missing or refused. Participants often load the platform
 * over plain HTTP on the event network, which is not a secure context, so the
 * fallback is the common path there, not an edge case.
 */
export async function writeTextToClipboard(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back to document copy for browsers or harnesses without clipboard permission.
    }
  }

  const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  // A dialog's focus trap pulls focus back from anything outside it, and
  // execCommand then reports success without copying the text.
  const host =
    previousFocus?.closest<HTMLElement>('[role="dialog"], dialog') ??
    document.querySelector<HTMLElement>('[role="dialog"][aria-modal="true"], dialog[open]') ??
    document.body;

  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.setAttribute("aria-hidden", "true");
  textarea.tabIndex = -1;
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  textarea.style.pointerEvents = "none";
  host.appendChild(textarea);
  textarea.focus({ preventScroll: true });
  textarea.select();

  const selected =
    document.activeElement === textarea &&
    textarea.selectionEnd - textarea.selectionStart === text.length;
  const copied = selected && document.execCommand("copy");
  textarea.remove();
  previousFocus?.focus({ preventScroll: true });

  if (!copied) {
    throw new Error("clipboard write failed");
  }
}
