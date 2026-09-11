import { useEffect, useState } from "react";
import {
  useSettings,
  useSettingsPatch,
} from "@/_lib/services/queries/settings.queries";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { CheckCircle, Globe, Zap } from "lucide-react";
import { SettingInput, SettingSelect, SettingToggle } from "./controls";
import { isValidProxy } from "./proxy";

/**
 * Post-download actions + proxy (PATCH /settings): open the folder when a
 * download finishes, an optional system action (shutdown / sleep / close), and
 * the outbound proxy URL. Toggle/select persist instantly; the proxy field
 * validates locally and commits on blur.
 */
export function PostDownloadSettings() {
  const { data: settings, isLoading, isError } = useSettings();
  const { patch, savedField, errorFor } = useSettingsPatch();

  const [proxy, setProxy] = useState<string>("");

  useEffect(() => {
    if (!settings) return;
    setProxy(settings.proxy ?? "");
  }, [settings]);

  const proxyInvalid = !isValidProxy(proxy);

  const commitProxy = (value: string) => {
    const next = value.trim();
    setProxy(next);
    if (!isValidProxy(next)) return; // leave the inline message up; don't save
    if (next === (settings?.proxy ?? "")) return;
    patch("proxy", { proxy: next }, {
      onError: () => setProxy(settings?.proxy ?? ""),
    });
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
              error={errorFor("post_download.auto_open_dir")}
            />

            <SettingSelect
              id="post-download-action"
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
              error={errorFor("post_download.action")}
              hint="Resets to Nothing when Rum restarts."
            />

            <Separator className="my-1" />

            <div className="flex items-center gap-2 text-lg font-semibold">
              <Globe className="w-5 h-5" /> Proxy
            </div>

            <SettingInput
              id="post-download-proxy"
              label="Proxy server"
              icon={<Globe className="w-4 h-4" />}
              value={proxy}
              placeholder="http://user:pass@host:port"
              hint="Leave empty for a direct connection."
              onChange={(e) => setProxy(e.target.value)}
              onBlur={() => commitProxy(proxy)}
              saved={savedField === "proxy"}
              error={
                proxyInvalid
                  ? "Use host:port, or an http/https/socks5 URL."
                  : errorFor("proxy")
              }
            />
          </>
        )}
      </CardContent>
    </Card>
  );
}

export default PostDownloadSettings;
