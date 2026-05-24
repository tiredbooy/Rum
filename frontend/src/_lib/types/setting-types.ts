export interface Setting {
  confirm_on_exit?: boolean;
  silent?: boolean;
  out_dir?: string;
  speed_limit_kb?: number;
  max_parallel?: number;
  max_retries?: number;
  preferred_theme: "system" | "light" | "dark";
}

export interface SettingReq extends Partial<Setting> {
  post_download: {
    auto_open_dir?: boolean;
    action?: "none" | "shutdown" | "sleep" | "close";
  };
  file_confilict?: "rename" | "overwrite" | "skip";
  proxy?: string;
}
