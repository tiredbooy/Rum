"use client";

import * as React from "react";
import { cn } from "@/lib/utils";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
  SheetFooter,
} from "@/components/ui/sheet";
import type { DialogProps } from "@radix-ui/react-dialog";

// Map size to Tailwind max-width classes
const sizeClasses: Record<string, string> = {
  sm: "sm:max-w-sm",
  md: "sm:max-w-md",
  lg: "sm:max-w-lg",
  xl: "sm:max-w-xl",
  "2xl": "sm:max-w-2xl",
  full: "sm:max-w-full sm:m-4", // stays slightly inset on large screens
};

interface ResponsiveDialogProps extends DialogProps {
  trigger?: React.ReactNode;
  title?: string;
  description?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  contentClassName?: string;
  size?: "sm" | "md" | "lg" | "xl" | "2xl" | "full"; // new prop
}

export function ResponsiveDialog({
  trigger,
  title,
  description,
  children,
  footer,
  contentClassName,
  size = "lg", // default matches your original max-w-lg
  open,
  onOpenChange,
  ...props
}: ResponsiveDialogProps) {
  const isDesktop = useMediaQuery("(min-width: 768px)");

  const maxWidthClass = sizeClasses[size];

  // Mobile sheet height: full screen when size="full", otherwise 85vh
  const sheetHeightClass =
    size === "full" ? "h-screen rounded-none" : "h-[85vh] rounded-t-xl";

  if (isDesktop) {
    return (
      <Dialog open={open} onOpenChange={onOpenChange} {...props}>
        {trigger && <DialogTrigger asChild>{trigger}</DialogTrigger>}
        <DialogContent
          className={cn(
            "bg-background text-foreground border-border",
            maxWidthClass, // dynamic width
            "max-h-[90vh] flex flex-col",
            contentClassName,
          )}
        >
          <DialogHeader className="shrink-0">
            {title && <DialogTitle>{title}</DialogTitle>}
            {description && (
              <DialogDescription>{description}</DialogDescription>
            )}
          </DialogHeader>
          <div className="py-2 overflow-y-auto overflow-x-hidden min-h-0 flex-1 min-w-0">
            {children}
          </div>
          {footer && <DialogFooter className="shrink-0">{footer}</DialogFooter>}
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} {...props}>
      {trigger && <SheetTrigger asChild>{trigger}</SheetTrigger>}
      <SheetContent
        side="bottom"
        className={cn(
          "bg-background text-foreground border-border",
          sheetHeightClass, // dynamic height
          "flex flex-col",
          contentClassName,
        )}
      >
        <SheetHeader className="text-left shrink-0">
          {title && <SheetTitle>{title}</SheetTitle>}
          {description && <SheetDescription>{description}</SheetDescription>}
        </SheetHeader>
        <div className="py-4 overflow-y-auto overflow-x-hidden min-h-0 flex-1 min-w-0">
          {children}
        </div>
        {footer && <SheetFooter className="shrink-0">{footer}</SheetFooter>}
      </SheetContent>
    </Sheet>
  );
}
