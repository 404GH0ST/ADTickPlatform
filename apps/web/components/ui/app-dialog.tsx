'use client';

import type { ReactElement, ReactNode } from 'react';

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
      <DialogContent className={contentClassName}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {body}
        {footer ? <DialogFooter>{footer}</DialogFooter> : null}
      </DialogContent>
    </Dialog>
  );
}
