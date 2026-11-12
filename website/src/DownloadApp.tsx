import { SiteShell } from "./components/layout/SiteShell";
import { DownloadPage } from "./pages/Download";

// The download page (download/index.html).
export default function DownloadApp() {
  return (
    <SiteShell page="download">
      <DownloadPage />
    </SiteShell>
  );
}
