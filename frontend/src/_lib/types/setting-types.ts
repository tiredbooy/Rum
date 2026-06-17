export interface Setting {
  confirm_on_exit?: boolean;
  silent?: boolean;
  out_dir?: string;
  speed_limit_kb?: number;
  max_parallel?: number;
  max_retries?: number;
  preferred_theme: "system" | "light" | "dark";
  // Desktop preferences (persisted via PATCH /api/v1/settings). snake_case to
  // match the Go json tags on config.Setting.
  launch_on_startup?: boolean;
  minimize_to_tray?: boolean;
  close_to_tray?: boolean;
  enable_clipboard_watch?: boolean;
}

export interface SettingReq extends Partial<Setting> {
  post_download: {
    auto_open_dir?: boolean;
    action?: "none" | "shutdown" | "sleep" | "close";
  };
  file_confilict?: "rename" | "overwrite" | "skip";
  proxy?: string;
}

/* ------------------------------------------------------------------ */
/* Bandwidth schedule (GET|PUT /api/v1/settings/schedule)             */
/* ------------------------------------------------------------------ */

/**
 * A single bandwidth-window rule. JSON is snake_case on the wire:
 *   { start_hour, end_hour, limit_kbps, days? }
 * - start_hour / end_hour: 0–23 local hours (wrap-around allowed, e.g. 22→6).
 * - limit_kbps: kB/s cap, 0 = unlimited.
 * - days: 0=Sun..6=Sat; empty/omitted = every day.
 */
export interface SpeedRule {
  start_hour: number;
  end_hour: number;
  limit_kbps: number;
  days?: number[];
}

export interface ScheduleSettings {
  scheduled_start_enabled: boolean;
  rules: SpeedRule[];
}

/* ------------------------------------------------------------------ */
/* Categories (GET|PUT /api/v1/settings/categories)                   */
/* ------------------------------------------------------------------ */

/**
 * A category auto-organize rule. JSON: { name, extensions, dest_dir }.
 * - extensions: lower-case, dot-prefixed (e.g. ".mp4").
 * - dest_dir: absolute, or relative to the configured download directory.
 */
export interface CategoryRule {
  name: string;
  extensions: string[];
  dest_dir: string;
}

export interface CategorySettings {
  enabled: boolean;
  rules: CategoryRule[];
}
