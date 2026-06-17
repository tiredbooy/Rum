import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { FolderTree } from "lucide-react";

interface CategoryBadgeProps {
  category?: string;
  className?: string;
}

/**
 * Small read-only badge surfacing a download's auto-organize category.
 * Renders nothing when no category is set. Mirrors the PriorityBadge pattern.
 */
export function CategoryBadge({ category, className }: CategoryBadgeProps) {
  if (!category) return null;
  return (
    <Badge
      variant="outline"
      className={cn(
        "gap-1 px-1.5 py-0 text-[10px] font-medium text-violet-400 border-violet-400/40 bg-violet-400/10",
        className,
      )}
    >
      <FolderTree className="h-3 w-3" />
      {category}
    </Badge>
  );
}

export default CategoryBadge;
