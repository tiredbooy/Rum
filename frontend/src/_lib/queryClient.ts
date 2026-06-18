import { QueryClient } from "@tanstack/react-query";

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 5_000,
        // Collect idle/stale query caches after a minute to keep RSS down.
        gcTime: 60_000,
        refetchOnWindowFocus: true,
        retry: 2,
      },
      mutations: {
        retry: 1,
      },
    },
  });
}
