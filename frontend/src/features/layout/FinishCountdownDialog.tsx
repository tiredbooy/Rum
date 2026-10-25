import { useEffect, useState } from "react";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  useCancelFinishAction,
  useFinishAction,
} from "@/_lib/services/queries/finish-action.queries";
import { countdownSeconds, finishTitle } from "@/_lib/finish-countdown";

/**
 * Shown while the backend counts down to sleep or shut down after the queue
 * finished. Dismissing it in any way (button, Esc) cancels the countdown.
 */
export function FinishCountdownDialog() {
  const { data: pending } = useFinishAction();
  const cancel = useCancelFinishAction();
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!pending) return;
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(id);
  }, [pending]);

  return (
    <AlertDialog
      open={!!pending}
      onOpenChange={(open) => {
        if (!open) cancel.mutate();
      }}
    >
      <AlertDialogContent size="sm">
        <AlertDialogHeader>
          <AlertDialogTitle>
            {pending
              ? finishTitle(pending.action, countdownSeconds(pending.deadline_ms, now))
              : ""}
          </AlertDialogTitle>
          <AlertDialogDescription>All downloads finished.</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel autoFocus>Cancel</AlertDialogCancel>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export default FinishCountdownDialog;
