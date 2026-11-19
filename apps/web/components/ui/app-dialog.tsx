'use client';

import { useRef, type ReactElement, type ReactNode, type RefObject } from 'react';

import { cn } from '@/lib/utils';

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ScrollableRegion } from '@/components/ui/scrollable-region';

type AppDialogProps = {
  body: ReactNode;
  contentClassName?: string;
  description: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
  open: boolean;
  pinFormFooter?: boolean;
  returnFocusRef?: RefObject<HTMLElement | null>;
  showCloseButton?: boolean;
  title: ReactNode;
};

export function AppDialog({
  body,
  contentClassName,
  description,
  footer,
  onClose,
  open,
  pinFormFooter = false,
  returnFocusRef,
  showCloseButton = true,
  title,
}: AppDialogProps): ReactElement {
  const opener = useRef<HTMLElement | null>(null);
  const titleElement = useRef<HTMLHeadingElement>(null);
  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
      <DialogContent
        showCloseButton={showCloseButton}
        onOpenAutoFocus={(event) => {
          const active = document.activeElement;
          opener.current = returnFocusRef?.current ?? (active instanceof HTMLElement && active !== document.body ? active : null);
          const dialog = event.currentTarget;
          if (!(dialog instanceof HTMLElement)) return;

          const fields = dialog.querySelectorAll<HTMLElement>(
            'input:not([type="hidden"]):enabled, select:enabled, textarea:enabled',
          );
          const field = Array.from(fields).find((element) => element.getClientRects().length > 0);
          if (field) {
            event.preventDefault();
            field.focus();
            if (field instanceof HTMLInputElement) field.select();
          } else if (dialog.scrollHeight > dialog.clientHeight && titleElement.current) {
            event.preventDefault();
            titleElement.current.focus({ preventScroll: true });
          }
        }}
        onCloseAutoFocus={(event) => {
          const target = opener.current;
          event.preventDefault();
          if (target?.isConnected) {
            target.focus();
            if (document.activeElement === target) return;
          }

          const fallback = document.querySelector<HTMLElement>('main h1, header h1') ?? document.querySelector('main');
          if (fallback) {
            if (!fallback.hasAttribute('tabindex')) fallback.tabIndex = -1;
            fallback.focus();
          }
        }}
        onFocusCapture={(event) => {
          // The dialog's focus trap can wrap focus without scrolling the target into view.
          if (event.target !== event.currentTarget) {
            event.target.scrollIntoView({ block: 'nearest', inline: 'nearest' });
          }
        }}
        className={cn(
          'max-h-[min(88dvh,48rem)] auto-rows-max overflow-y-auto [&_:focus-visible]:scroll-my-2',
          pinFormFooter && '[@media(height>=32rem)]:grid-rows-[auto_minmax(0,1fr)_auto] [@media(height>=32rem)]:overflow-hidden',
          contentClassName,
        )}
      >
        <DialogHeader>
          <DialogTitle
            ref={titleElement}
            tabIndex={-1}
            className="pe-9 wrap-anywhere"
          >
            {title}
          </DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {pinFormFooter ? (
          <ScrollableRegion
            axis="vertical"
            label="Dialog details"
            className="min-w-0 [@media(height>=32rem)]:min-h-0 [@media(height>=32rem)]:overflow-y-auto [@media(height>=32rem)]:pr-1"
          >
            <div>{body}</div>
          </ScrollableRegion>
        ) : (
          <div className="min-w-0">{body}</div>
        )}
        {footer ? (
          <DialogFooter className="border-t border-border/70 pt-4">
            {footer}
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
