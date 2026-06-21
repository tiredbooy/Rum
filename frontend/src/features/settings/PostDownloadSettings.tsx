import { useEffect, useState } from "react";
import {
  useSettings,
  useUpdateSettings,
} from "@/_lib/services/queries/settings.queries";
import type { SettingReq } from "@/_lib/types/setting-types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { CheckCircle, Globe, Zap } from "lucide-react";
import {
  SettingInput,
  SettingSelect,
  SettingToggle,
  useSavedFlash,
} from "./controls";

/**
 * Post-download actions + proxy (PATCH /settings): open the folder when a
 * download finishes, an optional system action (shutdown / sleep / close), and
 * the outbound proxy URL. Toggle/select persist instantly; the proxy field
 * commits on blur. Self-contained, so it no longer shares form state with the
 * rest of the page.
 */
export function PostDownloadSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, flash] = useSavedFlash();

  const [proxy, setProxy] = useState<string>("");

  useEffect(() => {
    if (!settings) return;
    setProxy(settings.proxy ?? "");
  }, [settings]);

  const patch = (field: string, payload: Partial<SettingReq>) => {
    updateMutation.mutate(payload, { onSuccess: () => flash(field) });
  };

  const commitProxy = (value: string) => {
    setProxy(value);
    if (value !== (settings?.proxy ?? "")) {
      patch("proxy", { proxy: value });
    }
  };

  const autoOpenDir = settings?.post_download?.auto_open_dir ?? false;
  const action = settings?.post_download?.action ?? "none";

  return (
    <Card className="col-span-full md:col-span-1">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-lg">
          <CheckCircle className="w-5 h-5" /> Post‑download
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        {isLoading ? (
          <p className="text-muted-foreground animate-pulse text-sm">
            Loading post‑download preferences…
          </p>
        ) : isError ? (
          <p className="text-destructive text-sm">
            Could not load post‑download preferences.
          </p>
        ) : (
          <>
            <SettingToggle
              id="post-download-auto-open"
              label="Open folder after finish"
              checked={autoOpenDir}
              onCheckedChange={(v) =>
                patch("post_download.auto_open_dir", {
                  post_download: { auto_open_dir: v },
                })
              }
              saved={savedField === "post_download.auto_open_dir"}
            />

            <SettingSelect
              label="System action"
              icon={<Zap className="w-4 h-4" />}
              value={action}
              options={[
                { value: "none", label: "Nothing" },
                { value: "shutdown", label: "Shutdown" },
                { value: "sleep", label: "Sleep" },
                { value: "close", label: "Close App" },
              ]}
              onValueChange={(v) =>
                patch("post_download.action", {
                  post_download: {
                    action: v as "none" | "shutdown" | "sleep" | "close",
                  },
                })
              }
              saved={savedField === "post_download.action"}
            />

            <Separator className="my-1" />

            <div className="flex items-center gap-2 text-lg font-semibold">
              <Globe className="w-5 h-5" /> Proxy
            </div>

            <SettingInput
              label="Proxy server"
              icon={<Globe className="w-4 h-4" />}
              value={proxy}
              placeholder="http://user:pass@host:port"
              onChange={(e) => setProxy(e.target.value)}
              onBlur={() => commitProxy(proxy)}
              saved={savedField === "proxy"}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default PostDownloadSettings;
