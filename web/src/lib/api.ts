export type HeatCell = { day: string; reviews: number; retained: number };
export type ForecastCell = { day: string; due: number };

export type Dashboard = {
  due_count: number;
  new_today: number;
  streak_current: number;
  north_star: number;
  heatmap: HeatCell[];
  tomorrow_forecast: number;
};

export type Stats = {
  reviewed_today: number;
  distribution: { again: number; hard: number; good: number; easy: number };
  forecast: ForecastCell[];
  heatmap: HeatCell[];
  streak_current: number;
  streak_best: number;
  retention: number;
  total_retained: number;
  north_star: number;
};

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`/api/v1${path}`, { credentials: "include" });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const body = await res.json();
  return body.data as T;
}

export const fetchDashboard = () => getJSON<Dashboard>("/progress/dashboard");
export const fetchStats = () => getJSON<Stats>("/progress/stats");
