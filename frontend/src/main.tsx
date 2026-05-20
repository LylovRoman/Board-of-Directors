import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles.css";

const mobileAppUrl = import.meta.env.VITE_MOBILE_APP_URL;

function isMobileDevice() {
  const userAgent = window.navigator.userAgent.toLowerCase();
  const touchMac = window.navigator.platform === "MacIntel" && window.navigator.maxTouchPoints > 1;
  return touchMac || /android|iphone|ipad|ipod|mobile|windows phone/.test(userAgent);
}

if (mobileAppUrl && isMobileDevice()) {
  window.location.replace(mobileAppUrl);
} else {
  ReactDOM.createRoot(document.getElementById("root")!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  );
}
