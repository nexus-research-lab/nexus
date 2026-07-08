import { App } from "@/App";
import { applyDesktopEntryRoute } from "@/bootstrap/desktop-entry-route";
import { bootstrapReactApp } from "@/bootstrap/root-bootstrap";

applyDesktopEntryRoute("/app");
bootstrapReactApp(() => <App />);
