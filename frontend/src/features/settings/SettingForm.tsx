// components/SettingsForm.tsx
import { useState, useEffect } from "react";
import {
  useSettings,
  useUpdateSettings,
} from "@/_lib/services/queries/settings.queries";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import {
  FolderOpen,
  Gauge,
  Download,
  Repeat,
  Monitor,
  Zap,
  Globe,
  FileWarning,
  CheckCircle,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { SettingReq } from "@/_lib/types/setting-types";

// Safe defaults – prevents any undefined field from crashing
const defaultSettings: SettingReq = {
  confirm_on_exit: false,
  silent: false,
  out_dir: "",
  speed_limit_kb: 0,
  max_parallel: 3,
  max_retries: 3,
  preferred_theme: "system",
  post_download: { auto_open_dir: false, action: "none" },
  file_confilict: "rename",
  proxy: "",
};

export function SettingsForm() {
  const { data: settings, isLoading, isError } = useSettings();
  const updateMutation = useUpdateSettings();
  const [savedField, setSavedField] = useState<string | null>(null);

  const [form, setForm] = useState<SettingReq>(defaultSettings);

  // Deep merge: if API returns undefined for some fields, defaults remain
  useEffect(() => {
    if (settings) {
      setForm({
        ...defaultSettings,
        ...settings,
        post_download: {
          ...defaultSettings.post_download,
          ...(settings.post_download || {}),
        },
      });
    }
  }, [settings]);

  // Generic update for top-level fields
  const updateField = <K extends keyof SettingReq>(
    field: K,
    value: SettingReq[K],
  ) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  // Update nested post_download subfield
  const updatePost = <K extends keyof SettingReq["post_download"]>(
    field: K,
    value: SettingReq["post_download"][K],
  ) => {
    setForm((prev) => ({
      ...prev,
      post_download: { ...prev.post_download, [field]: value },
    }));
  };

  // onBlur handler – sends only the changed field
  const handleBlur = (fieldName: string, payload: Partial<SettingReq>) => {
    updateMutation.mutate(payload, {
      onSuccess: () => {
        setSavedField(fieldName);
        setTimeout(() => setSavedField(null), 2000);
      },
    });
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-muted-foreground animate-pulse">Loading settings…</p>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-destructive">
          Could not load settings. Please try again.
        </p>
      </div>
    );
  }

  return (
    <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
      {/* General Section */}
      <Card className="col-span-full md:col-span-1">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Monitor className="w-5 h-5" /> General
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <SettingToggle
            label="Confirm on exit"
            checked={form.confirm_on_exit}
            onCheckedChange={(v) => updateField("confirm_on_exit", v)}
            onBlur={() =>
              handleBlur("confirm_on_exit", {
                confirm_on_exit: form.confirm_on_exit,
              })
            }
            saved={savedField === "confirm_on_exit"}
          />

          <SettingToggle
            label="Silent notifications"
            checked={form.silent}
            onCheckedChange={(v) => updateField("silent", v)}
            onBlur={() => handleBlur("silent", { silent: form.silent })}
            saved={savedField === "silent"}
          />

          <SettingSelect
            label="Theme"
            icon={<Monitor className="w-4 h-4" />}
            value={form.preferred_theme}
            options={[
              { value: "system", label: "System" },
              { value: "light", label: "Light" },
              { value: "dark", label: "Dark" },
            ]}
            onValueChange={(v) =>
              updateField(
                "preferred_theme",
                v as "system" | "light" | "dark" | undefined,
              )
            }
            onBlur={() =>
              handleBlur("preferred_theme", {
                preferred_theme: form.preferred_theme,
              })
            }
            saved={savedField === "preferred_theme"}
          />

          <SettingInput
            label="Save location"
            icon={<FolderOpen className="w-4 h-4" />}
            value={form.out_dir}
            placeholder="~/Downloads"
            onChange={(e) => updateField("out_dir", e.target.value)}
            onBlur={() => handleBlur("out_dir", { out_dir: form.out_dir })}
            saved={savedField === "out_dir"}
          />
        </CardContent>
      </Card>

      {/* Downloads Section */}
      <Card className="col-span-full md:col-span-1">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <Download className="w-5 h-5" /> Downloads
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <SettingInput
            label="Speed limit (kB/s)"
            icon={<Gauge className="w-4 h-4" />}
            type="number"
            min={0}
            value={form.speed_limit_kb}
            placeholder="Unlimited"
            onChange={(e) =>
              updateField(
                "speed_limit_kb",
                e.target.value === "" ? 0 : Number(e.target.value),
              )
            }
            onBlur={() =>
              handleBlur("speed_limit_kb", {
                speed_limit_kb: form.speed_limit_kb,
              })
            }
            saved={savedField === "speed_limit_kb"}
          />

          <SettingInput
            label="Parallel downloads"
            icon={<Zap className="w-4 h-4" />}
            type="number"
            min={1}
            max={20}
            value={form.max_parallel}
            onChange={(e) =>
              updateField(
                "max_parallel",
                e.target.value === "" ? 1 : Number(e.target.value),
              )
            }
            onBlur={() =>
              handleBlur("max_parallel", { max_parallel: form.max_parallel })
            }
            saved={savedField === "max_parallel"}
          />

          <SettingInput
            label="Retry attempts"
            icon={<Repeat className="w-4 h-4" />}
            type="number"
            min={0}
            max={10}
            value={form.max_retries}
            onChange={(e) =>
              updateField(
                "max_retries",
                e.target.value === "" ? 0 : Number(e.target.value),
              )
            }
            onBlur={() =>
              handleBlur("max_retries", { max_retries: form.max_retries })
            }
            saved={savedField === "max_retries"}
          />

          <SettingSelect
            label="If file exists"
            icon={<FileWarning className="w-4 h-4" />}
            value={form.file_confilict}
            options={[
              { value: "rename", label: "Rename" },
              { value: "overwrite", label: "Overwrite" },
              { value: "skip", label: "Skip" },
            ]}
            onValueChange={(v) =>
              updateField(
                "file_confilict",
                v as "rename" | "overwrite" | "skip" | undefined,
              )
            }
            onBlur={() =>
              handleBlur("file_confilict", {
                file_confilict: form.file_confilict,
              })
            }
            saved={savedField === "file_confilict"}
          />
        </CardContent>
      </Card>

      {/* Post‑download & Proxy */}
      <Card className="col-span-full md:col-span-1">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-lg">
            <CheckCircle className="w-5 h-5" /> Post‑download
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <SettingToggle
            label="Open folder after finish"
            checked={form.post_download.auto_open_dir}
            onCheckedChange={(v) => updatePost("auto_open_dir", v)}
            onBlur={() =>
              handleBlur("post_download.auto_open_dir", {
                post_download: {
                  auto_open_dir: form.post_download.auto_open_dir,
                },
              })
            }
            saved={savedField === "post_download.auto_open_dir"}
          />

          <SettingSelect
            label="System action"
            icon={<Zap className="w-4 h-4" />}
            value={form.post_download.action}
            options={[
              { value: "none", label: "Nothing" },
              { value: "shutdown", label: "Shutdown" },
              { value: "sleep", label: "Sleep" },
              { value: "close", label: "Close App" },
            ]}
            onValueChange={(v) =>
              updatePost(
                "action",
                v as "none" | "shutdown" | "sleep" | "close" | undefined,
              )
            }
            onBlur={() =>
              handleBlur("post_download.action", {
                post_download: { action: form.post_download.action },
              })
            }
            saved={savedField === "post_download.action"}
          />
        </CardContent>

        <Separator className="my-2" />

        <CardHeader className="pt-0">
          <CardTitle className="flex items-center gap-2 text-lg">
            <Globe className="w-5 h-5" /> Proxy
          </CardTitle>
        </CardHeader>
        <CardContent>
          <SettingInput
            label="Proxy server"
            icon={<Globe className="w-4 h-4" />}
            value={form.proxy}
            placeholder="http://user:pass@host:port"
            onChange={(e) => updateField("proxy", e.target.value)}
            onBlur={() => handleBlur("proxy", { proxy: form.proxy })}
            saved={savedField === "proxy"}
          />
        </CardContent>
      </Card>
    </div>
  );
}

/* ---------- Reusable sub‑components (unchanged, but included for completeness) ---------- */

function SettingToggle({
  label,
  checked,
  onCheckedChange,
  onBlur,
  saved,
}: {
  label: string;
  checked?: boolean;
  onCheckedChange: (v: boolean) => void;
  onBlur: () => void;
  saved: boolean;
}) {
  return (
    <div className="flex items-center justify-between">
      <Label className="text-sm">{label}</Label>
      <div className="flex items-center gap-2">
        {saved && (
          <Badge variant="outline" className="text-green-600 border-green-600">
            Saved
          </Badge>
        )}
        <Switch
          checked={checked ?? false}
          onCheckedChange={onCheckedChange}
          onBlur={onBlur}
        />
      </div>
    </div>
  );
}

function SettingInput({
  label,
  icon,
  saved,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  icon: React.ReactNode;
  saved: boolean;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="flex items-center gap-2 text-sm">
        {icon}
        {label}
      </Label>
      <div className="relative">
        <Input {...props} className={cn("pr-16", props.className)} />
        {saved && (
          <Badge
            variant="outline"
            className="absolute right-2 top-1/2 -translate-y-1/2 text-green-600 border-green-600"
          >
            Saved
          </Badge>
        )}
      </div>
    </div>
  );
}

function SettingSelect({
  label,
  icon,
  value,
  options,
  onValueChange,
  onBlur,
  saved,
}: {
  label: string;
  icon: React.ReactNode;
  value?: string;
  options: { value: string; label: string }[];
  onValueChange: (value: string) => void;
  onBlur: () => void;
  saved: boolean;
}) {
  return (
    <div className="space-y-1.5">
      <Label className="flex items-center gap-2 text-sm">
        {icon}
        {label}
      </Label>
      <div className="flex items-center gap-2">
        <Select
          value={value ?? ""}
          onValueChange={onValueChange}
          onOpenChange={(open) => {
            if (!open) onBlur();
          }}
        >
          <SelectTrigger className="w-full">
            <SelectValue placeholder="Select..." />
          </SelectTrigger>
          <SelectContent>
            {options.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {saved && (
          <Badge
            variant="outline"
            className="text-green-600 border-green-600 shrink-0"
          >
            Saved
          </Badge>
        )}
      </div>
    </div>
  );
}
