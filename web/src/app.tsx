import { AdminRoute } from "./routes/AdminRoute";
import { SetupRoute } from "./routes/SetupRoute";

export function App() {
  if (window.location.pathname.startsWith("/setup")) {
    return <SetupRoute />;
  }
  return <AdminRoute />;
}
