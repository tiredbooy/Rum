import { useRef } from "react";
import { Copy, Puzzle, RefreshCw } from "lucide-react";
import {
  useExtensionToken,
  useRegenerateExtensionToken,
} from "@/_lib/services/queries/extension.queries";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/lib/toast";
import { copyText } from "./pairing";

/**
 * Pairing token for the Rum browser extension. The extension sends it with every
 * request; paste it into the extension's options page. Regenerating cuts off any
 * extension still holding the old one.
 */
export function BrowserExtensionSettings() {
  const { data, isLoading, isError, refetch } = useExtensionToken();
  const regenerate = useRegenerateExtensionToken();
  const inputRef = useRef<HTMLInputElement>(null);

  const copy = async () => {
    if (!data) return;
    const input = inputRef.current;
    const ok = await copyText(data.token, navigator.clipboard, () => {
      input?.select();
      return document.execCommand("copy");
    });
    if (ok) toast.success("Copied");
    else toast.error("Copy failed. Select the token and copy it.");
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <Puzzle className="w-5 h-5" /> Browser extension
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading token…
          </p>
        ) : isError || !data ? (
          <div className="flex items-center justify-between gap-3">
            <p className="text-destructive text-sm">Could not load the token.</p>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              Retry
            </Button>
          </div>
        ) : (
          <>
            <div className="space-y-1.5">
              <Label htmlFor="extension-token">Pairing token</Label>
              <div className="flex gap-2">
                <Input
                  id="extension-token"
                  ref={inputRef}
                  readOnly
                  value={data.token}
                  className="font-mono text-xs"
                  onFocus={(e) => e.currentTarget.select()}
                />
                <Button variant="outline" onClick={copy}>
                  <Copy className="w-4 h-4" /> Copy
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                Paste it in the extension options.
              </p>
            </div>

            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm" disabled={regenerate.isPending}>
                  <RefreshCw className="w-4 h-4" /> Regenerate
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Regenerate token?</AlertDialogTitle>
                  <AlertDialogDescription>
                    The paired extension stops working until you paste the new
                    token.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={() =>
                      regenerate.mutate(undefined, {
                        onSuccess: () => toast.success("New token ready"),
                        onError: () => toast.error("Could not regenerate. Try again."),
                      })
                    }
                  >
                    Regenerate
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default BrowserExtensionSettings;
