import { SettingsForm } from "@/features/settings/SettingForm";

export default function Settings() {
  return (
    <div className="p-6">
      <h1 className="text-2xl font-semibold mb-6">Settings</h1>
      <SettingsForm />
    </div>
  );
}
