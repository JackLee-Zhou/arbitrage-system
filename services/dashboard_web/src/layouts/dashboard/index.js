// ============================================================================
// Overview — Precision Trading Terminal Dashboard
// ============================================================================

import { Link as RouterLink } from "react-router-dom";
import Grid from "@mui/material/Grid";
import { Card, Box } from "@mui/material";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import useTradingStore from "stores/trading";
import useSystemStore from "stores/system";

import { COLORS, GLOW } from "theme/tokens";

import { IoTrendingUp, IoWallet, IoStatsChart, IoPulse } from "react-icons/io5";

// ---- Thin accent border helper ---------------------------------------------
const accentBorder = {
  position: "relative",
  "&::after": {
    content: '""',
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    height: "1px",
    background: `linear-gradient(90deg, ${COLORS.CYAN}44, transparent 70%)`,
    borderRadius: "inherit",
  },
};

// ---- Stat block ------------------------------------------------------------
function StatBlock({ label, value, sub, accent, icon, to }) {
  const linkProps = to ? { component: RouterLink, to } : {};
  return (
    <Card
      {...linkProps}
      sx={{
        p: 2.5,
        background: COLORS.BG_SURFACE,
        border: `1px solid ${COLORS.BORDER_SUBTLE}`,
        transition: "border-color 200ms, box-shadow 200ms",
        cursor: to ? "pointer" : "default",
        textDecoration: "none",
        "&:hover": to
          ? {
              border: `1px solid ${COLORS.BORDER_ACTIVE}`,
              boxShadow: GLOW.CARD_HOVER,
            }
          : {},
        ...accentBorder,
      }}
    >
      <VuiBox display="flex" alignItems="center" gap={1} mb={1}>
        <VuiBox
          sx={{
            width: 36,
            height: 36,
            borderRadius: "8px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            background: `linear-gradient(135deg, ${accent}22, ${accent}08)`,
            color: accent,
            fontSize: "18px",
          }}
        >
          {icon}
        </VuiBox>
        <VuiTypography variant="caption" color="text" fontWeight="medium" textTransform="uppercase" letterSpacing="0.5px">
          {label}
        </VuiTypography>
      </VuiBox>
      <VuiTypography variant="h4" color="white" fontWeight="bold" mb={0.5}
        sx={{ fontFamily: "'Roboto Mono', monospace", letterSpacing: "-0.5px" }}>
        {value}
      </VuiTypography>
      {sub != null && (typeof sub === 'string' ? (
        <VuiTypography variant="caption" color="text">{sub}</VuiTypography>
      ) : sub)}
    </Card>
  );
}

// ---- Dot indicator ----------------------------------------------------------
function Dot({ color }) {
  return (
    <Box
      sx={{
        width: 6,
        height: 6,
        borderRadius: "50%",
        bgcolor: color,
        boxShadow: `0 0 6px ${color}`,
        display: "inline-block",
        mr: 0.5,
      }}
    />
  );
}

// ============================================================================
function Overview() {
  const { positions, totalFloatPnL, aiScores, todayOrders, todayTrades } = useTradingStore();
  const { services, systemMode, alerts, wsStatus } = useSystemStore();

  const positionCount = Object.keys(positions).length;
  const scoreEntries = Object.entries(aiScores);
  const liveScore = scoreEntries.find(([, v]) => v.type !== "shadow");
  const serviceList = Object.entries(services);
  const healthyCount = serviceList.filter(([, s]) => s.status === "healthy").length;
  const totalServices = serviceList.length || 9;

  const wsColor = wsStatus === "connected" ? COLORS.EMERALD : wsStatus === "connecting" ? COLORS.AMBER : COLORS.CRIMSON;

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        {/* ---- Top stat row ---- */}
        <VuiBox mb={3}>
          <Grid container spacing={2}>
            <Grid item xs={6} md={3}>
              <StatBlock
                label="当前价格"
                value="---"
                sub="等待行情"
                accent={COLORS.CYAN}
                icon={<IoTrendingUp />}
              />
            </Grid>
            <Grid item xs={6} md={3}>
              <StatBlock
                label="浮动盈亏"
                value={positionCount > 0 ? `¥${totalFloatPnL.toFixed(0)}` : "¥0"}
                sub={positionCount > 0 ? `${positionCount} 个合约` : "无持仓"}
                accent={totalFloatPnL >= 0 ? COLORS.EMERALD : COLORS.CRIMSON}
                icon={<IoWallet />}
                to="/futures/browse"
              />
            </Grid>
            <Grid item xs={6} md={3}>
              <StatBlock
                label="AI 评分"
                value={liveScore ? `${(liveScore[1].overallScore ?? liveScore[1].score ?? 0).toFixed(0)}` : "---"}
                sub={liveScore ? `${liveScore[0]}` : "等待数据"}
                accent={COLORS.VIOLET}
                icon={<IoStatsChart />}
                to="/ai/scores"
              />
            </Grid>
            <Grid item xs={6} md={3}>
              <StatBlock
                label="系统状态"
                value={`${healthyCount}/${totalServices}`}
                sub={
                  <VuiBox display="flex" alignItems="center" gap={1}>
                    <Dot color={wsColor} />
                    {wsStatus === "connected" ? "在线" : wsStatus === "connecting" ? "连接中" : "离线"}
                  </VuiBox>
                }
                accent={healthyCount === totalServices ? COLORS.EMERALD : COLORS.AMBER}
                icon={<IoPulse />}
                to="/system/nodes"
              />
            </Grid>
          </Grid>
        </VuiBox>

        {/* ---- Four-quadrant grid ---- */}
        <Grid container spacing={2}>
          {/* 左上 — 盯盘 */}
          <Grid item xs={12} md={6}>
            <Card sx={{ p: 3, background: COLORS.BG_SURFACE, border: `1px solid ${COLORS.BORDER_SUBTLE}`, ...accentBorder, height: "100%" }}>
              <VuiTypography variant="h6" color="white" fontWeight="bold" mb={2} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box sx={{ width: 8, height: 8, borderRadius: "2px", bgcolor: COLORS.CYAN, boxShadow: `0 0 6px ${COLORS.CYAN}` }} />
                盯盘区
              </VuiTypography>
              <Grid container spacing={1.5}>
                {[
                  ["今日委托", `${todayOrders.length} 笔`],
                  ["今日成交", `${todayTrades.length} 笔`],
                  ["系统模式", systemMode === "normal" ? "正常" : systemMode],
                  ["活跃持仓", `${positionCount} 个`],
                ].map(([label, value]) => (
                  <Grid item xs={6} key={label}>
                    <VuiTypography variant="caption" color="text" textTransform="uppercase" letterSpacing="0.5px">
                      {label}
                    </VuiTypography>
                    <VuiTypography variant="body1" color="white" fontWeight="medium" fontFamily="'Roboto Mono', monospace">
                      {value}
                    </VuiTypography>
                  </Grid>
                ))}
              </Grid>
            </Card>
          </Grid>

          {/* 右上 — AI */}
          <Grid item xs={12} md={6}>
            <Card sx={{ p: 3, background: COLORS.BG_SURFACE, border: `1px solid ${COLORS.BORDER_SUBTLE}`, ...accentBorder, height: "100%" }}>
              <VuiTypography variant="h6" color="white" fontWeight="bold" mb={2} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box sx={{ width: 8, height: 8, borderRadius: "2px", bgcolor: COLORS.VIOLET, boxShadow: `0 0 6px ${COLORS.VIOLET}` }} />
                AI 区
              </VuiTypography>
              {scoreEntries.length === 0 ? (
                <VuiTypography variant="body2" color="text">等待 AI 评分数据接入…</VuiTypography>
              ) : (
                scoreEntries.slice(0, 3).map(([symbol, data]) => (
                  <VuiBox key={symbol} mb={1.5} display="flex" justifyContent="space-between" alignItems="center">
                    <VuiTypography variant="body2" color="white" fontWeight="medium">{symbol}</VuiTypography>
                    <VuiBox display="flex" alignItems="center" gap={1}>
                      <VuiTypography variant="body2" color={COLORS.VIOLET} fontWeight="bold" fontFamily="'Roboto Mono', monospace">
                        {(data.overallScore ?? data.score ?? 0).toFixed(0)}
                      </VuiTypography>
                      <VuiTypography variant="caption" color="text">{(data.confidence ?? 0) * 100}%</VuiTypography>
                    </VuiBox>
                  </VuiBox>
                ))
              )}
            </Card>
          </Grid>

          {/* 左下 — 系统 */}
          <Grid item xs={12} md={6}>
            <Card sx={{ p: 3, background: COLORS.BG_SURFACE, border: `1px solid ${COLORS.BORDER_SUBTLE}`, ...accentBorder, height: "100%" }}>
              <VuiTypography variant="h6" color="white" fontWeight="bold" mb={2} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box sx={{ width: 8, height: 8, borderRadius: "2px", bgcolor: COLORS.EMERALD, boxShadow: `0 0 6px ${COLORS.EMERALD}` }} />
                系统区
              </VuiTypography>
              <VuiBox display="flex" flexWrap="wrap" gap={1}>
                {[
                  "redis", "influxdb", "ctp_tick", "intel_collector", "llm_scorer",
                  "trading_agent", "orchestrator", "db_writer", "watchdog",
                ].map((name) => {
                  const svc = services[name];
                  const healthy = svc?.status === "healthy";
                  return (
                    <VuiBox
                      key={name}
                      sx={{
                        px: 1.5,
                        py: 0.5,
                        borderRadius: "4px",
                        fontSize: "12px",
                        fontFamily: "'Roboto Mono', monospace",
                        color: healthy ? COLORS.EMERALD : COLORS.AMBER,
                        background: healthy ? `${COLORS.EMERALD}10` : `${COLORS.AMBER}10`,
                        border: `1px solid ${healthy ? COLORS.EMERALD : COLORS.AMBER}22`,
                      }}
                    >
                      <Dot color={healthy ? COLORS.EMERALD : COLORS.AMBER} />
                      {name}
                    </VuiBox>
                  );
                })}
              </VuiBox>
            </Card>
          </Grid>

          {/* 右下 — 事件流 */}
          <Grid item xs={12} md={6}>
            <Card sx={{ p: 3, background: COLORS.BG_SURFACE, border: `1px solid ${COLORS.BORDER_SUBTLE}`, ...accentBorder, height: "100%" }}>
              <VuiTypography variant="h6" color="white" fontWeight="bold" mb={2} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <Box sx={{ width: 8, height: 8, borderRadius: "2px", bgcolor: COLORS.AMBER, boxShadow: `0 0 6px ${COLORS.AMBER}` }} />
                实时事件
              </VuiTypography>
              {alerts.length === 0 ? (
                <VuiTypography variant="body2" color="text">暂无事件，系统运行正常</VuiTypography>
              ) : (
                alerts.slice(0, 5).map((alert, i) => (
                  <VuiBox
                    key={i}
                    sx={{
                      py: 1,
                      borderBottom: i < Math.min(alerts.length, 5) - 1 ? `1px solid ${COLORS.BORDER_SUBTLE}` : "none",
                    }}
                  >
                    <VuiTypography variant="caption" color="text" fontFamily="'Roboto Mono', monospace">
                      {alert.timestamp ? new Date(alert.timestamp).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "--:--:--"}
                    </VuiTypography>
                    <VuiTypography variant="body2" color="white" ml={1} display="inline">
                      {alert.message || alert.title || JSON.stringify(alert)}
                    </VuiTypography>
                  </VuiBox>
                ))
              )}
            </Card>
          </Grid>
        </Grid>
      </VuiBox>
    </DashboardLayout>
  );
}

export default Overview;
