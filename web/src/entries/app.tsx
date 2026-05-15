import { App } from "@/App";
import { apply_desktop_entry_route } from "@/bootstrap/desktop-entry-route";
import { bootstrap_react_app } from "@/bootstrap/root-bootstrap";

apply_desktop_entry_route("/app");
bootstrap_react_app(() => <App />);
