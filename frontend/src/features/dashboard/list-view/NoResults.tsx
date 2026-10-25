import { Button } from "@/components/ui/button";
import { SearchX } from "lucide-react";
import { useListView } from "./useListView";

/** Shown when the search hides every download. One action: clear filters. */
export function NoResults() {
  const clearFilters = useListView((s) => s.clearFilters);
  return (
    <div
      role="status"
      className="flex flex-col items-center justify-center gap-3 py-16 text-center text-muted-foreground"
    >
      <SearchX className="h-8 w-8" aria-hidden="true" />
      <p className="text-sm">No matches</p>
      <Button variant="outline" size="sm" onClick={clearFilters}>
        Clear filters
      </Button>
    </div>
  );
}
