import { lazy, Suspense } from "react";
import { RouterProvider } from "react-router";
import { router } from "./router";
import { createQueryClient } from "@/_lib/queryClient";
import { QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/sonner";
import { ErrorBoundary } from "@/components/error-boundary";
import { AllProgressStream } from "./hooks/useAllProgressStream";
import DropProvider from "@/features/drag-drop/DropProvider";

// Dev-only: the React Query devtools add a sizeable bundle + runtime footprint.
// Lazy-load them in development and drop them entirely from production builds.
const ReactQueryDevtools = import.meta.env.DEV
  ? lazy(() =>
      import("@tanstack/react-query-devtools").then((m) => ({
        default: m.ReactQueryDevtools,
      })),
    )
  : () => null;

const queryClient = createQueryClient();

function App() {
  return (
    <ErrorBoundary>
      <Toaster />
      <QueryClientProvider client={queryClient}>
        <AllProgressStream />
        <DropProvider />
        <RouterProvider router={router} />
        {import.meta.env.DEV && (
          <Suspense fallback={null}>
            <ReactQueryDevtools initialIsOpen={false} />
          </Suspense>
        )}
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
