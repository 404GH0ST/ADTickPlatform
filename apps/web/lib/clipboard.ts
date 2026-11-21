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

  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  textarea.style.pointerEvents = "none";
  document.body.appendChild(textarea);
  textarea.focus();
  textarea.select();

  const copied = document.execCommand("copy");
  document.body.removeChild(textarea);

  if (!copied) {
    throw new Error("clipboard write failed");
  }
}
