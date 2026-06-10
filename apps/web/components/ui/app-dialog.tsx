'use client';

import type { ReactElement, ReactNode } from 'react';

import { cn } from '@/lib/utils';

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

type AppDialogProps = {
  body: ReactNode;
  contentClassName?: string;
  description: ReactNode;
  footer?: ReactNode;
  onClose: () => void;
  open: boolean;
  title: ReactNode;
};

export function AppDialog({
  body,
  contentClassName,
  description,
  footer,
  onClose,
  open,
  title,
}: AppDialogProps): ReactElement {
  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
      <DialogContent
        className={cn(
          'grid-rows-[auto_minmax(0,1fr)_auto] max-h-[min(88vh,48rem)] overflow-hidden',
          contentClassName,
        )}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto pr-1">{body}</div>
        {footer ? (
          <DialogFooter className="border-t border-border/70 pt-4">
            {footer}
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
