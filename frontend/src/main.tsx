import React from "react";
import ReactDOM from "react-dom/client";
import { AnimateUIProvider } from "@animate-ui/react/provider";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode><AnimateUIProvider tempo={0.7} reducedMotion="never"><App /></AnimateUIProvider></React.StrictMode>,
);
