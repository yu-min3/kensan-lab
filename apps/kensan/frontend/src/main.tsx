import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AppShell } from "./components/AppShell";
import { Dashboard } from "./pages/Dashboard";
import { DailyPage } from "./pages/DailyPage";
import { MemoPage } from "./pages/MemoPage";
import { TasksPage } from "./pages/TasksPage";
import { ProjectsPage } from "./pages/ProjectsPage";
import { ReviewsPage } from "./pages/ReviewsPage";
import { NotesPage } from "./pages/NotesPage";
import { LifeGoalsPage } from "./pages/LifeGoalsPage";
import { TrashPage } from "./pages/TrashPage";
import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 15_000, refetchOnWindowFocus: true },
  },
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppShell>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/tasks" element={<TasksPage />} />
            <Route path="/projects" element={<ProjectsPage />} />
            <Route path="/daily" element={<DailyPage />} />
            <Route path="/memos" element={<MemoPage />} />
            <Route path="/notes" element={<NotesPage />} />
            <Route path="/life" element={<LifeGoalsPage />} />
            <Route path="/reviews" element={<ReviewsPage />} />
            <Route path="/trash" element={<TrashPage />} />
          </Routes>
        </AppShell>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
);
