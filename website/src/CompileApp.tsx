import { SiteShell } from "./components/layout/SiteShell";
import { PlaygroundPage } from "./pages/Playground";

// The playground page (compile/index.html).
export default function CompileApp() {
  return (
    <SiteShell page="compile">
      <PlaygroundPage />
    </SiteShell>
  );
}
