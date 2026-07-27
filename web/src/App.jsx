import { useEffect, useState } from "react";

export default function App() {
  const [apiStatus, setApiStatus] = useState("checking...");

  useEffect(() => {
    fetch("/api/healthz")
      .then((res) => res.json())
      .then((data) => setApiStatus(data.status ?? "unknown"))
      .catch(() => setApiStatus("unreachable"));
  }, []);

  return (
    <div style={{ fontFamily: "sans-serif", padding: "2rem" }}>
      <h1>OAADrive</h1>
      <p>Private family backup &amp; media vault.</p>
      <p>API status: <strong>{apiStatus}</strong></p>
    </div>
  );
}
