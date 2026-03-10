import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppLayout } from "@/layouts/AppLayout";
import { AgentsPage } from "@/pages/AgentsPage";
import { ChatPage } from "@/pages/ChatPage";
import { CreateFlowPage } from "@/pages/CreateFlowPage";
import { CreateProjectPage } from "@/pages/CreateProjectPage";
import { DashboardPage } from "@/pages/DashboardPage";
import { ExecutionDetailPage } from "@/pages/ExecutionDetailPage";
import { FlowDetailPage } from "@/pages/FlowDetailPage";
import { FlowsPage } from "@/pages/FlowsPage";
import { ProjectsPage } from "@/pages/ProjectsPage";

interface AppProps {
  a2aEnabledOverride?: boolean;
  uiVersionOverride?: "v2" | "v3";
}

const App = (_props: AppProps = {}) => {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/chat" element={<ChatPage />} />
          <Route path="/flows" element={<FlowsPage />} />
          <Route path="/flows/new" element={<CreateFlowPage />} />
          <Route path="/flows/:flowId" element={<FlowDetailPage />} />
          <Route path="/executions/:execId" element={<ExecutionDetailPage />} />
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="/projects" element={<ProjectsPage />} />
          <Route path="/projects/new" element={<CreateProjectPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
};

export default App;
