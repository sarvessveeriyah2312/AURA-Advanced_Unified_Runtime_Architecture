import { SiteShell } from "./components/layout/SiteShell";
import { ChangelogPage } from "./pages/Changelog";

// The changelog page (changelog/index.html).
export default function ChangelogApp() {
  return (
    <SiteShell page="changelog">
      <ChangelogPage />
    </SiteShell>
  );
}
