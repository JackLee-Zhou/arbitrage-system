// ============================================================================
// index.js — arbitrage-system Dashboard Entry
// Based on Vision UI Dashboard React (MIT License)
// ============================================================================

import React from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "App";

// Vision UI Dashboard React Context Provider
import { VisionUIControllerProvider } from "context";

const container = document.getElementById("root");
const root = createRoot(container);

root.render(
  <BrowserRouter>
    <VisionUIControllerProvider>
      <App />
    </VisionUIControllerProvider>
  </BrowserRouter>
);
