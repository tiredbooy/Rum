import { createBrowserRouter } from "react-router-dom";
import { lazy, Suspense } from "react";
import Layout from "./features/layout/Layout";

// Route-level code splitting: each page is its own chunk, loaded on demand so
// the initial bundle (and idle memory) stays small.
const Dashboard = lazy(() => import("./pages/Dashboard"));
const Settings = lazy(() => import("./pages/Settings"));
const ActiveDownloads = lazy(() => import("./pages/ActiveDownloads"));
const CompletedDownloads = lazy(() => import("./pages/CompletedDownloads"));
const FailedDownloads = lazy(() => import("./pages/FailedDownloads"));

const wrap = (el: React.ReactNode) => <Suspense fallback={null}>{el}</Suspense>;

export const router = createBrowserRouter([
  {
    path: "/",
    element: <Layout />,
    children: [
      { index: true, element: wrap(<Dashboard />) },
      { path: "active", element: wrap(<ActiveDownloads />) },
      { path: "completed", element: wrap(<CompletedDownloads />) },
      { path: "failed", element: wrap(<FailedDownloads />) },
      { path: "settings", element: wrap(<Settings />) },
    ],
  },
]);
