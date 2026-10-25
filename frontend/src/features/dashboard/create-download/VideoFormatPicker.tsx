import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { optionLabel } from "@/_lib/video-format";
import type { VideoOption } from "@/_lib/types/video-types";

/** Short quality list for one video: Best, 1080p, 720p, 480p, Audio only. */
export function VideoFormatPicker({
  options,
  value,
  onChange,
}: {
  options: VideoOption[];
  value: string;
  onChange: (key: string) => void;
}) {
  return (
    <RadioGroup value={value} onValueChange={onChange} aria-label="Quality" className="gap-2">
      {options.map((o) => (
        <div key={o.key} className="flex items-center gap-2">
          <RadioGroupItem value={o.key} id={`video-format-${o.key}`} />
          <Label htmlFor={`video-format-${o.key}`} className="text-sm font-normal">
            {optionLabel(o)}
          </Label>
        </div>
      ))}
    </RadioGroup>
  );
}
