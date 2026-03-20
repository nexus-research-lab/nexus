import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import "./app/globals.css";

const container = document.getElementById("root");
if (!container) {
  throw new Error("Root container #root not found.");
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
