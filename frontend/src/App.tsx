import { RouterProvider } from "react-router";
import { router } from "./router";
import { createQueryClient } from "@/_lib/queryClient";
import { QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "@/components/ui/sonner";
import { ErrorBoundary } from "@/components/error-boundary";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { AllProgressStream } from "./hooks/useAllProgressStream";

const queryClient = createQueryClient();

function App() {
  return (
    <ErrorBoundary>
      <Toaster />
      <QueryClientProvider client={queryClient}>
        <AllProgressStream />
        <RouterProvider router={router} />
        <ReactQueryDevtools initialIsOpen={false} />
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
