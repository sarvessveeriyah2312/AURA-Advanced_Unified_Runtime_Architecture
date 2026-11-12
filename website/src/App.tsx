import { SiteShell } from "./components/layout/SiteShell";
import { Home } from "./pages/Home";

// The home page (index.html). The changelog page is ChangelogApp.
export default function App() {
  return (
    <SiteShell page="home">
      <Home />
    </SiteShell>
  );
}
