import { RouterProvider } from "react-router";
import { router } from "./router";
import { createQueryClient } from "@/_lib/queryClient";
import { QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { AllProgressStream } from "./hooks/useAllProgressStream";

const queryClient = createQueryClient();

function App() {
  return (
    <>
      <Toaster position="top-right" richColors />
      <QueryClientProvider client={queryClient}>
        <AllProgressStream />
        <RouterProvider router={router} />
        <ReactQueryDevtools initialIsOpen={false} />
      </QueryClientProvider>
    </>
  );
}

export default App;
