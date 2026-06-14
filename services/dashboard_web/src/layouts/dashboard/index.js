// ============================================================================
// Overview — 概览首页（三区汇总）
// OPT-302 将实现完整三区卡片布局
// ============================================================================

import Grid from "@mui/material/Grid";
import { Card } from "@mui/material";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import MiniStatisticsCard from "examples/Cards/StatisticsCards/MiniStatisticsCard";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import { IoWallet } from "react-icons/io5";
import { IoGlobe } from "react-icons/io5";
import { IoStatsChart } from "react-icons/io5";
import { IoTrendingUp } from "react-icons/io5";

function Overview() {
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
                count="---"
                percentage={{ color: "text", text: "等待数据" }}
                icon={{ color: "info", component: <IoTrendingUp size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "浮动盈亏", fontWeight: "regular" }}
                count="¥0"
                percentage={{ color: "text", text: "今日" }}
                icon={{ color: "info", component: <IoWallet size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "AI 评分", fontWeight: "regular" }}
                count="---"
                percentage={{ color: "text", text: "等待数据" }}
                icon={{ color: "info", component: <IoStatsChart size="22px" color="white" /> }}
              />
            </Grid>
            <Grid item xs={12} md={6} xl={3}>
              <MiniStatisticsCard
                title={{ text: "服务健康", fontWeight: "regular" }}
                count="0/9"
                percentage={{ color: "text", text: "等待巡检" }}
                icon={{ color: "info", component: <IoGlobe size="22px" color="white" /> }}
              />
            </Grid>
          </Grid>
        </VuiBox>

        {/* ---- 三区卡片占位 ---- */}
        <VuiBox mb={3}>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>
                  📊 盯盘区
                </VuiTypography>
                <VuiTypography variant="body2" color="text">
                  当前价格 + 持仓 + 今日委托/成交（OPT-302 实现）
                </VuiTypography>
              </Card>
            </Grid>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>
                  🤖 AI 区
                </VuiTypography>
                <VuiTypography variant="body2" color="text">
                  Live AI 评分 + 老化时间 + 紧急事件（OPT-302 实现）
                </VuiTypography>
              </Card>
            </Grid>
          </Grid>
        </VuiBox>

        <VuiBox mb={3}>
          <Grid container spacing={3}>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>
                  ⚙️ 系统区
                </VuiTypography>
                <VuiTypography variant="body2" color="text">
                  9 个服务健康灯 + 内存/磁盘使用率（OPT-302 实现）
                </VuiTypography>
              </Card>
            </Grid>
            <Grid item xs={12} md={6}>
              <Card sx={{ p: 3 }}>
                <VuiTypography variant="h6" color="white" fontWeight="bold" mb={1}>
                  📰 实时事件流
                </VuiTypography>
                <VuiTypography variant="body2" color="text">
                  WebSocket 推送的订单/告警/AI 事件（OPT-302 实现）
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
