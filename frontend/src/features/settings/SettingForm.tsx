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
import { CategoryManager } from "./CategoryManager";
import { DesktopSettings } from "./DesktopSettings";

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
      </div>

      {/* Bandwidth schedule + scheduled-start (PUT /settings/schedule) */}
      <ScheduleEditor />

      {/* Category auto-organize (PUT /settings/categories) */}
      <CategoryManager />

      {/* Desktop preferences (PATCH /settings) */}
      <DesktopSettings />
    </div>
  );
}
