import { createBrowserRouter } from "react-router-dom";
import Layout from "./features/layout/Layout";
import Dashboard from "./pages/Dashboard";
import Settings from "./pages/Settings";
// import ActiveDownloads from "./pages/ActiveDownloads";
// import CompletedDownloads from "./pages/CompletedDownloads";
// import FailedDownloads from "./pages/FailedDownloads";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <Layout />,
    children: [
      { index: true, element: <Dashboard /> },
      // { path: "active", element: <ActiveDownloads /> },
      //   { path: "completed", element: <CompletedDownloads /> },
      //   { path: "failed", element: <FailedDownloads /> },
      { path: "settings", element: <Settings /> },
    ],
  },
]);
