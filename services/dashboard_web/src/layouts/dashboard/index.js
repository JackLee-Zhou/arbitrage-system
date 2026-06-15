// ============================================================================
// Overview — 概览首页（三区汇总）— 接入真实 API
// ============================================================================

import { useQuery } from "@tanstack/react-query";
import { fetchOverview } from "api/client";
import Grid from "@mui/material/Grid";
import { Card } from "@mui/material";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import MiniStatisticsCard from "examples/Cards/StatisticsCards/MiniStatisticsCard";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import { IoWallet, IoGlobe, IoStatsChart, IoTrendingUp } from "react-icons/io5";

const M = { fontFamily: "'Roboto Mono', 'Courier New', monospace" };

function Overview() {
  const { data, isLoading } = useQuery({
    queryKey: ["overview"],
    queryFn: fetchOverview,
    refetchInterval: 15000, // refresh every 15s
  });

  const services = data?.services || {};
  const positions = data?.positions?.positions || data?.positions || [];
  const posCount = Array.isArray(positions) ? positions.length : Object.keys(positions).length;
  const aiScore = data?.ai_score;
  const engine = data?.engine;
  const todayStats = data?.today_stats || { orders: 0, trades: 0, pnl: 0 };
  const serviceList = Object.entries(services);
  const healthyCount = serviceList.filter(([, s]) => s === "healthy" || s?.status === "healthy").length;
  const totalServices = serviceList.length || 9;

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        {/* ---- Top KPI row ---- */}
        <VuiBox mb={3}>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "系统状态", fontWeight: "regular" }}
                count={isLoading ? "..." : data?.status === "ok" ? "在线" : "离线"}
                percentage={{
                  color: data?.status === "ok" ? "success" : "error",
                  text: `${healthyCount}/${totalServices} 服务`,
                }}
                icon={{ color: "info", component: <IoGlobe size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "持仓", fontWeight: "regular" }}
                count={posCount > 0 ? `${posCount} 个` : "无"}
                percentage={{
                  color: posCount > 0 ? "success" : "text",
                  text: posCount > 0 ? "活跃持仓" : "等待数据",
                }}
                icon={{ color: "info", component: <IoWallet size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "AI 评分", fontWeight: "regular" }}
                count={aiScore ? (aiScore.score ?? aiScore.Score ?? "---") : "---"}
                percentage={{
                  color: "info",
                  text: aiScore ? "置信度 " + ((aiScore.confidence ?? 0) * 100).toFixed(0) + "%" : "等待数据",
                }}
                icon={{ color: "info", component: <IoStatsChart size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "今日统计", fontWeight: "regular" }}
                count={todayStats.orders > 0 ? `${todayStats.orders}笔` : "0笔"}
                percentage={{
                  color: todayStats.pnl >= 0 ? "success" : "error",
                  text: `委托${todayStats.orders} / 成交${todayStats.trades}`,
                }}
                icon={{ color: "info", component: <IoTrendingUp size="22px" color="white" /> }}
              />
            </Grid>
          </Grid>
        </VuiBox>

        {/* ---- Three-zone cards ---- */}
        <VuiBox mb={3}>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>📊 盯盘区</VuiTypography>
                {engine ? (
                  <Grid container spacing={1}>
                    {[
                      ["系统模式", engine.mode || "---"],
                      ["当前价格", "---"],
                      ["今日委托", `${todayStats.orders} 笔`],
                      ["今日成交", `${todayStats.trades} 笔`],
                    ].map(([label, value]) => (
                      <Grid item xs={6} key={label}>
                        <VuiTypography variant="caption" color="text">{label}</VuiTypography>
                        <VuiTypography variant="body2" color="white" fontWeight="medium" sx={M}>
                          {value}
                        </VuiTypography>
                      </Grid>
                    ))}
                  </Grid>
                ) : (
                  <VuiTypography variant="body2" color="text">
                    {isLoading ? "加载中..." : "等待数据 — orchestrator API 未连接"}
                  </VuiTypography>
                )}
              </Card>
            </Grid>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>🤖 AI 区</VuiTypography>
                {aiScore ? (
                  <VuiBox>
                    <VuiTypography variant="h4" color="info" fontWeight="bold" sx={M}>
                      {aiScore.score ?? aiScore.Score ?? "---"}
                    </VuiTypography>
                    <VuiTypography variant="body2" color="text" mt={1}>
                      Label: {aiScore.label ?? aiScore.Label ?? "---"} · Horizon: {aiScore.horizon ?? "---"} · Confidence: {((aiScore.confidence ?? 0) * 100).toFixed(0)}%
                    </VuiTypography>
                  </VuiBox>
                ) : (
                  <VuiTypography variant="body2" color="text">
                    {isLoading ? "加载中..." : "等待 AI 评分数据..."}
                  </VuiTypography>
                )}
              </Card>
            </Grid>
          </Grid>
        </VuiBox>

        <VuiBox mb={3}>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>⚙️ 系统区</VuiTypography>
                {serviceList.length > 0 ? (
                  <VuiBox display="flex" flexWrap="wrap" gap={0.8}>
                    {["redis", "influxdb", "ctp_tick", "intel_collector", "llm_scorer", "trading_agent", "orchestrator", "db_writer", "watchdog"].map((name) => {
                      const svc = services[name];
                      const healthy = svc === "healthy" || svc?.status === "healthy";
                      return (
                        <VuiBox key={name} sx={{
                          px: 1.5, py: 0.5, borderRadius: "4px", fontSize: "12px",
                          color: healthy ? "#01b574" : "#ffb547",
                          background: healthy ? "rgba(1,181,116,0.1)" : "rgba(255,181,71,0.1)",
                          border: `1px solid ${healthy ? "rgba(1,181,116,0.2)" : "rgba(255,181,71,0.2)"}`,
                        }}>
                          {name}
                        </VuiBox>
                      );
                    })}
                  </VuiBox>
                ) : (
                  <VuiTypography variant="body2" color="text">
                    {isLoading ? "加载中..." : "等待服务数据..."}
                  </VuiTypography>
                )}
              </Card>
            </Grid>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>📰 实时事件</VuiTypography>
                <VuiTypography variant="body2" color="text">
                  WebSocket 事件流将在 orchestrator /ws 就绪后接入
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
