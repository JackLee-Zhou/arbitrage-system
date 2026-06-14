// OPT-306: AI 评分页 — Live + Shadow 评分实时展示
import { useMemo } from "react";
import Grid from "@mui/material/Grid";
import Card from "@mui/material/Card";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiBadge from "components/VuiBadge";
import VuiProgress from "components/VuiProgress";
import { COLORS } from "theme/tokens";
import useTradingStore from "stores/trading";

const AGE_LABELS = { fresh: "新鲜", stale: "老化", expired: "过期" };
const AGE_COLORS = { fresh: "#00FF94", stale: COLORS.AI_STALE, expired: "#FF4D6D" };

function getAgeStatus(ts) {
  if (!ts) return "stale";
  const diff = Date.now() - new Date(ts).getTime();
  const min = diff / 60000;
  if (min < 15) return "fresh";
  if (min < 30) return "stale";
  return "expired";
}

const LABEL_COLORS = { bullish: "success", bearish: "error", neutral: "warning", range: "warning" };

function ScoreCard({ symbol, data }) {
  const score = data.overallScore ?? data.score;
  const label = data.overallLabel ?? data.label;
  const confidence = data.confidence ?? 0;
  const horizon = data.horizon ?? "---";
  const regime = data.regime ?? "---";
  const drivers = data.keyDrivers ?? [];
  const chain = data.modelChain ?? [];
  const isShadow = data.type === "shadow";
  const age = getAgeStatus(data.updatedAt ?? data.timestamp);

  return (
    <Card sx={{
      p: 2.5, opacity: isShadow ? 0.6 : 1,
      ...(isShadow ? {} : {
        "&::before": {
          content: '""', position: "absolute", inset: 0, borderRadius: "inherit", padding: "1px",
          background: `linear-gradient(135deg, ${COLORS.AI_SCORE}66, ${COLORS.AI_SCORE}22, transparent)`,
          mask: "linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)",
          maskComposite: "exclude", WebkitMaskComposite: "xor", pointerEvents: "none",
        },
      }),
    }}>
      <VuiBox display="flex" justifyContent="space-between" alignItems="center" mb={1.5}>
        <VuiTypography variant="h6" color="white" fontWeight="bold">{symbol}</VuiTypography>
        <VuiBox display="flex" alignItems="center" gap={0.5}>
          <VuiBox sx={{ width: 8, height: 8, borderRadius: "50%", bgcolor: AGE_COLORS[age], boxShadow: `0 0 6px ${AGE_COLORS[age]}` }} />
          <VuiTypography variant="caption" color="text">{AGE_LABELS[age]}</VuiTypography>
          {isShadow && <VuiBadge color="dark" size="xs" variant="standard">SHADOW</VuiBadge>}
        </VuiBox>
      </VuiBox>
      <VuiBox display="flex" alignItems="baseline" gap={1} mb={1}>
        <VuiTypography variant="h3" fontWeight="bold"
          sx={{ color: COLORS.AI_SCORE, textShadow: isShadow ? "none" : `0 0 20px ${COLORS.AI_SCORE}66` }}>
          {score != null ? score.toFixed(0) : "---"}
        </VuiTypography>
        {label && (
          <VuiBadge color={LABEL_COLORS[label] || "info"} size="xs">{label}</VuiBadge>
        )}
      </VuiBox>
      <VuiBox mb={1.5}>
        <VuiBox display="flex" justifyContent="space-between" mb={0.5}>
          <VuiTypography variant="caption" color="text">置信度</VuiTypography>
          <VuiTypography variant="caption" color="text">{(confidence * 100).toFixed(0)}%</VuiTypography>
        </VuiBox>
        <VuiProgress value={Math.round(confidence * 100)} color="info" />
      </VuiBox>
      <VuiBox display="flex" gap={2} mb={1.5}>
        <VuiTypography variant="caption" color="text">周期: <VuiTypography variant="caption" color="white" fontWeight="medium">{horizon}</VuiTypography></VuiTypography>
        <VuiTypography variant="caption" color="text">状态: <VuiTypography variant="caption" color="white" fontWeight="medium">{regime}</VuiTypography></VuiTypography>
      </VuiBox>
      {drivers.length > 0 && (
        <VuiBox mb={1.5}>
          <VuiTypography variant="caption" color="text" mb={0.5} display="block">关键驱动</VuiTypography>
          {drivers.map((d, i) => (
            <VuiTypography key={i} variant="caption" color="white" display="block"
              sx={{ pl: 1, borderLeft: `2px solid ${COLORS.AI_SCORE}44`, mb: 0.25 }}>{d}</VuiTypography>
          ))}
        </VuiBox>
      )}
      {chain.length > 0 && (
        <VuiBox display="flex" gap={0.5} flexWrap="wrap">
          {chain.map((m, i) => (
            <VuiBadge key={i} color="info" variant="standard" size="xs">{m}</VuiBadge>
          ))}
        </VuiBox>
      )}
    </Card>
  );
}

function AIScores() {
  const { aiScores } = useTradingStore();
  const entries = useMemo(() => Object.entries(aiScores), [aiScores]);

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox mb={3} display="flex" justifyContent="space-between" alignItems="center">
          <VuiTypography variant="h4" color="white" fontWeight="bold">AI 评分</VuiTypography>
          <VuiBox display="flex" gap={1} alignItems="center">
            {Object.entries(AGE_COLORS).map(([k, c]) => (
              <VuiBox key={k} display="flex" alignItems="center" gap={0.5}>
                <VuiBox sx={{ width: 8, height: 8, borderRadius: "50%", bgcolor: c, boxShadow: `0 0 6px ${c}` }} />
                <VuiTypography variant="caption" color="text">{AGE_LABELS[k]}</VuiTypography>
              </VuiBox>
            ))}
          </VuiBox>
        </VuiBox>
        {entries.length === 0 ? (
          <Card sx={{ p: 4, textAlign: "center" }}>
            <VuiTypography variant="body1" color="text">暂无 AI 评分数据，等待行情接入...</VuiTypography>
          </Card>
        ) : (
          <Grid container spacing={3}>
            {entries.map(([symbol, data]) => (
              <Grid item xs={12} md={6} xl={4} key={symbol}>
                <ScoreCard symbol={symbol} data={data} />
              </Grid>
            ))}
          </Grid>
        )}
      </VuiBox>
    </DashboardLayout>
  );
}

export default AIScores;
