// OPT-306: AI 评分历史页 — 曲线占位（Tremor LineChart 就绪后替换）
import { useState } from "react";
import Card from "@mui/material/Card";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiButton from "components/VuiButton";
const AI_SCORE = "#9D5FFF";

const PERIODS = ["1H", "4H", "1D", "1W"];

// Placeholder area chart — 数据点坐标 [cx, cy]
const DATA_POINTS = [
  [50, 160], [150, 140], [250, 110], [350, 100], [450, 130],
  [550, 90], [650, 80], [750, 100],
];

function buildD(pts) {
  const [first, ...rest] = pts;
  let d = `M${first[0]},${first[1]}`;
  for (let i = 0; i < rest.length; i++) {
    const prev = pts[i];
    const cp1x = (prev[0] + rest[i][0]) / 2;
    d += ` Q${cp1x},${prev[1]} ${rest[i][0]},${rest[i][1]}`;
  }
  return d;
}

function AIScoresHistory() {
  const [period, setPeriod] = useState("1H");

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox mb={3} display="flex" justifyContent="space-between" alignItems="center">
          <VuiTypography variant="h4" color="white" fontWeight="bold">AI 评分历史</VuiTypography>
          <VuiBox display="flex" gap={0.5}>
            {PERIODS.map((p) => (
              <VuiButton key={p} color={period === p ? "info" : "dark"} size="small"
                onClick={() => setPeriod(p)}
                sx={{ minWidth: 40, px: 1.5, color: period === p ? "white" : "text" }}>
                {p}
              </VuiButton>
            ))}
          </VuiBox>
        </VuiBox>
        <Card sx={{ p: 3 }}>
          <VuiBox sx={{ position: "relative", width: "100%", height: 380, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center" }}>
            <svg width="90%" height="260" viewBox="0 0 800 200" preserveAspectRatio="xMidYMid meet">
              <defs>
                <linearGradient id="areaFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={AI_SCORE} stopOpacity="0.3" />
                  <stop offset="100%" stopColor={AI_SCORE} stopOpacity="0" />
                </linearGradient>
              </defs>
              {[0, 25, 50, 75, 100].map((v, i) => (
                <line key={i} x1={50} y1={185 - v * 1.6} x2={750} y2={185 - v * 1.6}
                  stroke="rgba(255,255,255,0.05)" strokeWidth="1" />
              ))}
              <path d={`${buildD(DATA_POINTS)} L${DATA_POINTS[DATA_POINTS.length - 1][0]},200 L50,200 Z`}
                fill="url(#areaFill)" />
              <path d={buildD(DATA_POINTS)} fill="none" stroke={AI_SCORE} strokeWidth="2" strokeLinecap="round" />
              {DATA_POINTS.map(([cx, cy], i) => (
                <circle key={i} cx={cx} cy={cy} r="3.5" fill={AI_SCORE}
                  stroke="#1a1f37" strokeWidth="2" />
              ))}
            </svg>
            <VuiTypography variant="body2" color="text" sx={{ mt: 1, textAlign: "center" }}>
              历史数据将在 orchestrator API 就绪后接入
            </VuiTypography>
          </VuiBox>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default AIScoresHistory;
