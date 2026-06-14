// ============================================================================
// Overview — 概览首页（三区汇总）
// 顶部 KPI 卡片使用 useTradingStore / useSystemStore 实时数据
// OPT-302 将实现完整三区卡片布局
// ============================================================================

import { useMemo, useState, useEffect } from "react";
import { useHistory } from "react-router-dom";
import Grid from "@mui/material/Grid";
import { Card, Box } from "@mui/material";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import MiniStatisticsCard from "examples/Cards/StatisticsCards/MiniStatisticsCard";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import { IoWallet } from "react-icons/io5";
import { IoGlobe } from "react-icons/io5";
import { IoStatsChart } from "react-icons/io5";
import { IoTrendingUp } from "react-icons/io5";

import useTradingStore from "stores/trading";
import useSystemStore from "stores/system";

function Overview() {
  const history = useHistory();
  const { positions, totalFloatPnL, aiScores, todayOrders, todayTrades } = useTradingStore();
  const { services, systemMode, alerts } = useSystemStore();
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  // ---- 当前价格 ----
  const currentPrice = useMemo(() => {
    const posKeys = Object.keys(positions);
    if (posKeys.length === 0) return null;
    // 优先使用 selectedSymbol，否则取第一个
    const firstPos = positions[posKeys[0]];
    return firstPos?.lastPrice ?? firstPos?.price ?? null;
  }, [positions]);

  const priceLabel = useMemo(() => {
    const posKeys = Object.keys(positions);
    if (posKeys.length === 0) return "---";
    return currentPrice != null ? currentPrice.toFixed(1) : "---";
  }, [currentPrice, positions]);

  const pricePercentage = useMemo(() => {
    const posKeys = Object.keys(positions);
    if (posKeys.length === 0) return { color: "text", text: "等待数据" };
    const firstPos = positions[posKeys[0]];
    const changePct = firstPos?.changePct;
    if (changePct != null) {
      return {
        color: changePct >= 0 ? "success" : "error",
        text: `${changePct >= 0 ? "+" : ""}${changePct.toFixed(2)}%`,
      };
    }
    return { color: "text", text: posKeys[0] };
  }, [positions, currentPrice]);

  // ---- 浮动盈亏 ----
  const pnlDisplay = useMemo(() => {
    if (totalFloatPnL == null || totalFloatPnL === 0) return "¥0";
    const prefix = totalFloatPnL > 0 ? "+" : "";
    return `¥${prefix}${totalFloatPnL.toFixed(0)}`;
  }, [totalFloatPnL]);

  const pnlColor = useMemo(() => {
    if (totalFloatPnL == null || totalFloatPnL === 0) return "text";
    return totalFloatPnL > 0 ? "success" : "error";
  }, [totalFloatPnL]);

  const pnlText = useMemo(() => {
    if (totalFloatPnL == null || totalFloatPnL === 0) return "持平";
    return totalFloatPnL > 0 ? "浮盈" : "浮亏";
  }, [totalFloatPnL]);

  // ---- AI 评分 ----
  const aiScoreEntry = useMemo(() => {
    const keys = Object.keys(aiScores);
    if (keys.length === 0) return null;
    const firstKey = keys[0];
    const entry = aiScores[firstKey];
    if (!entry) return null;
    const score = entry.overallScore ?? entry.score;
    const label = entry.overallLabel ?? entry.label ?? firstKey;
    return { score, label, keyDrivers: entry.keyDrivers ?? [] };
  }, [aiScores]);

  const aiScoreDisplay = useMemo(() => {
    if (!aiScoreEntry || aiScoreEntry.score == null) return "---";
    return aiScoreEntry.score.toFixed(0);
  }, [aiScoreEntry]);

  const aiScorePercentage = useMemo(() => {
    if (!aiScoreEntry) return { color: "text", text: "等待数据" };
    const driver = aiScoreEntry.keyDrivers[0] || aiScoreEntry.label;
    return { color: "info", text: driver || "评分就绪" };
  }, [aiScoreEntry]);

  // ---- 服务健康 ----
  const serviceCount = useMemo(() => {
    const keys = Object.keys(services);
    if (keys.length === 0) return { healthy: 0, total: 9 };
    const healthy = keys.filter((k) => {
      const s = services[k];
      return s?.status === "online" || s?.status === "healthy" || s?.alive === true;
    }).length;
    return { healthy, total: Math.max(keys.length, 9) };
  }, [services]);

  const healthPercentage = useMemo(() => {
    const { healthy, total } = serviceCount;
    if (healthy === 0) return { color: "text", text: "等待巡检" };
    if (healthy === total) return { color: "success", text: "全部在线" };
    if (healthy >= total / 2) return { color: "warning", text: `${healthy}/${total} 在线` };
    return { color: "error", text: `${healthy}/${total} 在线` };
  }, [serviceCount]);

  // ---- 盯盘区 ----
  const orderCount = todayOrders?.length || 0;
  const tradeCount = todayTrades?.length || 0;
  const posEntries = Object.entries(positions || {});

  // ---- AI 老化 & 事件 ----
  const aiAgeSec = useMemo(() => {
    const ts = Object.values(aiScores).map((s) => (s.timestamp ? new Date(s.timestamp).getTime() : 0)).filter(Boolean);
    return ts.length ? Math.floor((now - Math.max(...ts)) / 1000) : null;
  }, [aiScores, now]);
  const aiAgeStr = aiAgeSec == null ? "---" : aiAgeSec < 60 ? `${aiAgeSec}s` : `${Math.floor(aiAgeSec / 60)}m ${aiAgeSec % 60}s`;
  const emergencies = useMemo(() => (alerts || []).filter((a) => a.level === "emergency" || a.severity === "high").slice(0, 3), [alerts]);
  const events = useMemo(() => (alerts || []).slice(0, 20), [alerts]);

  // ---- 系统区 ----
  const svcEntries = Object.entries(services || {});
  const isHealthy = (s) => s?.status === "online" || s?.status === "healthy" || s?.alive === true;
  const go = (p) => () => history.push(p);

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        {/* ---- 顶部 KPI 卡片 ---- */}
        <VuiBox mb={3}>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "当前价格", fontWeight: "regular" }}
                count={priceLabel}
                percentage={pricePercentage}
                icon={{ color: "info", component: <IoTrendingUp size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "浮动盈亏", fontWeight: "regular" }}
                count={pnlDisplay}
                percentage={{ color: pnlColor, text: pnlText }}
                icon={{ color: pnlColor === "error" ? "error" : "info", component: <IoWallet size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "AI 评分", fontWeight: "regular" }}
                count={aiScoreDisplay}
                percentage={aiScorePercentage}
                icon={{ color: "info", component: <IoStatsChart size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "服务健康", fontWeight: "regular" }}
                count={`${serviceCount.healthy}/${serviceCount.total}`}
                percentage={healthPercentage}
                icon={{ color: "info", component: <IoGlobe size="22px" color="white" /> }}
              />
            </Grid>
          </Grid>
        </VuiBox>

        {/* ---- 2x2 四区卡片 ---- */}
        <VuiBox mb={3}>
          <Grid container spacing={3}>
            {/* -- 📊 盯盘区 -- */}
            <Grid item xs={12} md={6}>
              <Card sx={{ height:"100%", display:"flex", flexDirection:"column", justifyContent:"space-between" }}>
                <VuiBox>
                  <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>📊 盯盘区</VuiTypography>
                  <VuiTypography variant="h3" color="white" fontWeight="bold">
                    {priceLabel}
                    <VuiTypography variant="button" color={pricePercentage.color} fontWeight="bold" ml={1}>{pricePercentage.text}</VuiTypography>
                  </VuiTypography>
                  <VuiBox mt={1}>
                    {posEntries.length === 0 ? (
                      <VuiTypography variant="caption" color="text">暂无持仓</VuiTypography>
                    ) : posEntries.slice(0, 3).map(([sym, pos]) => (
                      <VuiBox key={sym} display="flex" justifyContent="space-between">
                        <VuiTypography variant="body2" color="white">{sym}</VuiTypography>
                        <VuiTypography variant="body2" color={pos.floatPnL >= 0 ? "success" : "error"}>
                          {pos.volume||pos.net||0}手 {pos.floatPnL != null ? `${pos.floatPnL >= 0 ? "+" : ""}${pos.floatPnL.toFixed(0)}` : "---"}
                        </VuiTypography>
                      </VuiBox>
                    ))}
                  </VuiBox>
                  <VuiBox display="flex" gap={2} mt={0.5}>
                    <VuiTypography variant="caption" color="text">今日委托: {orderCount}</VuiTypography>
                    <VuiTypography variant="caption" color="text">今日成交: {tradeCount}</VuiTypography>
                  </VuiBox>
                </VuiBox>
                <VuiTypography onClick={go("/futures/browse")} variant="button" color="info" sx={{ cursor:"pointer", mt:1, display:"inline-flex", alignItems:"center" }}>
                  → 点击进入详细
                </VuiTypography>
              </Card>
            </Grid>

            {/* -- 🤖 AI 区 -- */}
            <Grid item xs={12} md={6}>
              <Card sx={{ height:"100%", display:"flex", flexDirection:"column", justifyContent:"space-between" }}>
                <VuiBox>
                  <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>🤖 AI 区</VuiTypography>
                  <VuiTypography variant="h2" color="white" fontWeight="bold">
                    {aiScoreDisplay}
                    <VuiTypography variant="button" color="info" fontWeight="bold" ml={1}>{aiScorePercentage.text}</VuiTypography>
                  </VuiTypography>
                  <VuiBox display="flex" alignItems="center" mt={1}>
                    <VuiTypography variant="caption" color="text" mr={1}>老化时间:</VuiTypography>
                    <VuiTypography variant="body2" color={aiAgeSec != null && aiAgeSec > 300 ? "warning" : "success"}>{aiAgeStr}</VuiTypography>
                  </VuiBox>
                  {emergencies.length > 0 && (
                    <VuiBox mt={1}>
                      <VuiTypography variant="caption" color="error">紧急事件 ({emergencies.length})</VuiTypography>
                      {emergencies.map((e, i) => (
                        <VuiTypography key={i} variant="caption" color="text" display="block">• {e.title || e.message || e.type}</VuiTypography>
                      ))}
                    </VuiBox>
                  )}
                </VuiBox>
                <VuiTypography onClick={go("/ai/scores")} variant="button" color="info" sx={{ cursor:"pointer", mt:1, display:"inline-flex", alignItems:"center" }}>
                  → 点击进入详细
                </VuiTypography>
              </Card>
            </Grid>

            {/* -- ⚙️ 系统区 -- */}
            <Grid item xs={12} md={6}>
              <Card sx={{ height:"100%", display:"flex", flexDirection:"column", justifyContent:"space-between" }}>
                <VuiBox>
                  <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>⚙️ 系统区</VuiTypography>
                  <VuiBox display="flex" flexWrap="wrap" gap={0.5}>
                    {svcEntries.length === 0 ? (
                      <VuiTypography variant="caption" color="text">等待服务注册</VuiTypography>
                    ) : svcEntries.map(([name, svc]) => (
                      <VuiBox key={name} display="flex" alignItems="center" mr={1.5} mb={0.5}>
                        <Box component="span" sx={{ width:8, height:8, borderRadius:"50%", display:"inline-block", mr:0.5, bgcolor: isHealthy(svc) ? "success.main" : svc?.status === "warning" ? "warning.main" : "error.main" }} />
                        <VuiTypography variant="caption" color="text">{name}</VuiTypography>
                      </VuiBox>
                    ))}
                  </VuiBox>
                  <VuiTypography variant="caption" color="text" mt={1} display="block">
                    系统模式: {systemMode} | 巡检: {serviceCount.healthy}/{serviceCount.total}
                  </VuiTypography>
                </VuiBox>
                <VuiTypography onClick={go("/system/nodes")} variant="button" color="info" sx={{ cursor:"pointer", mt:1, display:"inline-flex", alignItems:"center" }}>
                  → 点击进入详细
                </VuiTypography>
              </Card>
            </Grid>

            {/* -- 📰 实时事件流 -- */}
            <Grid item xs={12} md={6}>
              <Card sx={{ height:"100%", display:"flex", flexDirection:"column", justifyContent:"space-between" }}>
                <VuiBox>
                  <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>📰 实时事件流</VuiTypography>
                  <VuiBox sx={{ maxHeight:180, overflowY:"auto" }}>
                    {events.length === 0 ? (
                      <VuiTypography variant="body2" color="text">暂无事件</VuiTypography>
                    ) : events.map((e, i) => (
                      <VuiBox key={i} display="flex" alignItems="flex-start" mb={0.3}>
                        <VuiTypography variant="caption" color="text" sx={{ minWidth:55 }}>
                          {e.timestamp ? new Date(e.timestamp).toLocaleTimeString() : "---"}
                        </VuiTypography>
                        <VuiTypography variant="caption" color="white" ml={1}>
                          {e.title || e.message || e.type || "事件"}
                        </VuiTypography>
                      </VuiBox>
                    ))}
                  </VuiBox>
                </VuiBox>
                <VuiTypography onClick={go("/system/audit")} variant="button" color="info" sx={{ cursor:"pointer", mt:1, display:"inline-flex", alignItems:"center" }}>
                  → 点击进入详细
                </VuiTypography>
              </Card>
            </Grid>
          </Grid>
        </VuiBox>
      </VuiBox>
    </DashboardLayout>
  );
}

export default Overview;
