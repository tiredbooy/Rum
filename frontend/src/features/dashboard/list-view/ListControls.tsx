import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/native-select";
import { Search, X } from "lucide-react";
import { useEffect, useRef } from "react";
import { SORT_KEYS, SORT_LABELS, type SortKey } from "./list-view";
import { useListView } from "./useListView";

/** Search box and sort for the download list. Ctrl/Cmd+F focuses the search. */
export function ListControls() {
  const query = useListView((s) => s.query);
  const sort = useListView((s) => s.sort);
  const setQuery = useListView((s) => s.setQuery);
  const setSort = useListView((s) => s.setSort);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === "f") {
        e.preventDefault();
        inputRef.current?.focus();
        inputRef.current?.select();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className="flex flex-wrap items-center gap-2" role="search">
      <div className="relative w-full max-w-xs">
        <Search
          className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          ref={inputRef}
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape" && query) {
              e.stopPropagation();
              setQuery("");
            }
          }}
          placeholder="Search"
          aria-label="Search downloads by name or URL"
          className="pl-8 pr-8"
        />
        {query && (
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              setQuery("");
              inputRef.current?.focus();
            }}
            aria-label="Clear search"
            className="absolute right-0.5 top-1/2 size-8 -translate-y-1/2"
          >
            <X className="size-4" aria-hidden="true" />
          </Button>
        )}
      </div>
      <NativeSelect
        value={sort}
        onChange={(e) => setSort(e.target.value as SortKey)}
        aria-label="Sort downloads"
      >
        {SORT_KEYS.map((k) => (
          <option key={k} value={k}>
            {SORT_LABELS[k]}
          </option>
        ))}
      </NativeSelect>
    </div>
  );
}
