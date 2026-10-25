// components/SettingsForm.tsx
//
// Thin layout that composes the self-contained settings cards. Each card owns
// its own data (useSettings/useSchedule/useCategories) and persistence, so
// editing one card never re-renders the others — this is what keeps the page
// responsive (previously a single shared `form` state here re-rendered every
// heavy card on each keystroke). All cards save live, matching the Appearance
// card's behaviour.
import { GeneralSettings } from "./GeneralSettings";
import { DownloadSettings } from "./DownloadSettings";
import { PostDownloadSettings } from "./PostDownloadSettings";
import { IntegritySettings } from "./IntegritySettings";
import { AppearanceSettings } from "./AppearanceSettings";
import { StorageSettings } from "./StorageSettings";
import { ScheduleEditor } from "./ScheduleEditor";
import { NightPlanEditor } from "./NightPlanEditor";
import { CategoryManager } from "./CategoryManager";
import { DesktopSettings } from "./DesktopSettings";
import { BackupSettings } from "./BackupSettings";
import { YtdlpSettings } from "./YtdlpSettings";

import { BrowserExtensionSettings } from "./BrowserExtensionSettings";

export function SettingsForm() {
  return (
    <div className="space-y-6">
      <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
        {/* General: confirm-on-exit / silent / theme / save location */}
        <GeneralSettings />

        {/* Downloads: speed limit / parallel / connections / retries / conflict */}
        <DownloadSettings />

        {/* Post-download actions + proxy */}
        <PostDownloadSettings />

        {/* Integrity & reliability */}
        <IntegritySettings />

        {/* Appearance: accent / density / reduced-motion / sound */}
        <AppearanceSettings />

        {/* Storage: temp dir + keep-partial-on-failure */}
        <StorageSettings />

        {/* Video downloads: yt-dlp status, install/update, custom path */}
        <YtdlpSettings />
      </div>

      {/* Bandwidth schedule + scheduled-start (PUT /settings/schedule) */}
      <ScheduleEditor />

      {/* Night plan (PUT /settings/night-plan) */}
      <NightPlanEditor />

      {/* Category auto-organize (PUT /settings/categories) */}
      <CategoryManager />

      {/* Desktop preferences (PATCH /settings) */}
      <DesktopSettings />

      {/* Download list export/import + diagnostics */}
      <BackupSettings />

      {/* Browser-extension pairing token (GET/POST /extension/token) */}
      <BrowserExtensionSettings />
    </div>
  );
}
