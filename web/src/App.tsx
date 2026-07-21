import { useState } from "react";
import "./tokens.css";
import OfflineBanner from "./components/OfflineBanner";
import Dashboard from "./screens/Dashboard";
import Stats from "./screens/Stats";

const TABS = ["Trang chủ", "Ôn", "Thư viện", "Thống kê"];

export default function App() {
  const [tab, setTab] = useState(0);
  const [dark, setDark] = useState(false);
  const toggle = () => {
    const next = !dark;
    setDark(next);
    document.documentElement.setAttribute("data-theme", next ? "dark" : "light");
  };
  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <header style={{ padding: 16, display: "flex", justifyContent: "space-between" }}>
        <strong style={{ color: "var(--accent)" }}>MEMORIX</strong>
        <button aria-label="Đổi giao diện" onClick={toggle}>◑</button>
      </header>
      <OfflineBanner />
      <main style={{ flex: 1 }}>
        {tab === 0 && <Dashboard />}
        {tab === 3 && <Stats />}
        {(tab === 1 || tab === 2) && (
          <div style={{ padding: 16 }}>
            <h1>{TABS[tab]}</h1>
            <p style={{ color: "var(--muted)" }}>Màn do sprint khác triển khai.</p>
          </div>
        )}
      </main>
      <nav
        role="navigation"
        style={{ display: "flex", borderTop: "1px solid var(--line)", background: "var(--surface)" }}
      >
        {TABS.map((t, i) => (
          <button
            key={t}
            onClick={() => setTab(i)}
            aria-current={tab === i}
            style={{ flex: 1, minHeight: "var(--tap)", border: "none", background: "none", color: tab === i ? "var(--accent)" : "var(--muted)" }}
          >
            {t}
          </button>
        ))}
      </nav>
    </div>
  );
}
