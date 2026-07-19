// formatInterval đổi giây → nhãn tiếng Việt ngắn cho nút chấm (FR-14).
export function formatInterval(seconds: number): string {
  if (seconds < 3600) return `${Math.max(1, Math.round(seconds / 60))} phút`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)} giờ`;
  const days = Math.round(seconds / 86400);
  if (days < 30) return `${days} ngày`;
  if (days < 365) return `${Math.round(days / 30)} tháng`;
  return `${Math.round(days / 365)} năm`;
}
