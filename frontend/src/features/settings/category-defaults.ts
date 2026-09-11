import type { CategoryRule } from "@/_lib/types/setting-types";

/** Starter rule set offered by "Use defaults" in the category manager. */
export const DEFAULT_CATEGORY_RULES: CategoryRule[] = [
  {
    name: "Video",
    extensions: [".mp4", ".mkv", ".avi", ".mov", ".webm"],
    dest_dir: "Videos",
  },
  {
    name: "Audio",
    extensions: [".mp3", ".flac", ".wav", ".aac", ".ogg"],
    dest_dir: "Music",
  },
  {
    name: "Documents",
    extensions: [".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".txt"],
    dest_dir: "Documents",
  },
  {
    name: "Archives",
    extensions: [".zip", ".rar", ".7z", ".tar", ".gz"],
    dest_dir: "Archives",
  },
  {
    name: "Images",
    extensions: [".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"],
    dest_dir: "Images",
  },
];
