import { RouterProvider } from "react-router";
import { router } from "./router";

import { createQueryClient } from "@/_lib/queryClient";
import { QueryClientProvider } from "@tanstack/react-query";

const queryClient = createQueryClient();

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}

export default App;
