import { useEffect, useState } from "react";

export default function OfflineBanner() {
  const [offline, setOffline] = useState(false);
  useEffect(() => {
    const off = () => setOffline(true);
    const on = () => setOffline(false);
    window.addEventListener("offline", off);
    window.addEventListener("online", on);
    return () => {
      window.removeEventListener("offline", off);
      window.removeEventListener("online", on);
    };
  }, []);
  if (!offline) return null;
  return (
    <div role="status" style={{
      position: "sticky", top: 0, zIndex: 10, padding: "8px 16px",
      background: "var(--hard)", color: "#fff", textAlign: "center",
    }}>
      Mất mạng — điểm đã lưu, sẽ sync khi có kết nối.
    </div>
  );
}
