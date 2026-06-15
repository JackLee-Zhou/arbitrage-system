// ============================================================================
// 9 个服务健康详情 — OPT-307
// ============================================================================

import { useMemo } from "react";
import { useQuery } from '@tanstack/react-query';
import Grid from "@mui/material/Grid";
import Card from "@mui/material/Card";
import Box from "@mui/material/Box";
import Tooltip from "@mui/material/Tooltip";
import CircularProgress from "@mui/material/CircularProgress";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import { fetchServices } from 'api/client';

const SERVICE_NAMES = [
  "redis",
  "influxdb",
  "ctp_tick",
  "intel_collector",
  "llm_scorer",
  "trading_agent",
  "orchestrator",
  "db_writer",
  "watchdog",
];

const DISPLAY_NAMES = {
  redis: "Redis",
  influxdb: "InfluxDB",
  ctp_tick: "CTP Tick",
  intel_collector: "Intel Collector",
  llm_scorer: "LLM Scorer",
  trading_agent: "Trading Agent",
  orchestrator: "Orchestrator",
  db_writer: "DB Writer",
  watchdog: "Watchdog",
};

const STATUS_CONFIG = {
  healthy: { color: "success", label: "健康", hex: "#4CAF50" },
  degraded: { color: "warning", label: "降级", hex: "#FF9800" },
  unhealthy: { color: "error", label: "异常", hex: "#F44336" },
  unknown: { color: "grey", label: "未知", hex: "#9E9E9E" },
};

function getStatusInfo(service) {
  if (!service) return STATUS_CONFIG.unknown;
  const isHealthy =
    service.status === "online" ||
    service.status === "healthy" ||
    service.alive === true;
  if (isHealthy) return STATUS_CONFIG.healthy;
  if (service.status === "degraded" || service.status === "warning")
    return STATUS_CONFIG.degraded;
  if (
    service.status === "offline" ||
    service.status === "unhealthy" ||
    service.status === "error"
  )
    return STATUS_CONFIG.unhealthy;
  return STATUS_CONFIG.unknown;
}

function formatLastCheck(service) {
  const ts = service?.lastCheck ?? service?.lastSeen ?? service?.updatedAt;
  if (!ts) return "--:--:--";
  try {
    const d = new Date(ts);
    if (isNaN(d.getTime())) return "--:--:--";
    const pad = (n) => String(n).padStart(2, "0");
    return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  } catch {
    return "--:--:--";
  }
}

function SystemNodes() {
  const { data: services = {}, isLoading } = useQuery({
    queryKey: ['services'],
    queryFn: fetchServices,
    refetchInterval: 15000,
  });

  const serviceCards = useMemo(
    () =>
      SERVICE_NAMES.map((name) => {
        const svc = services[name];
        return {
          name,
          displayName: DISPLAY_NAMES[name],
          status: getStatusInfo(svc),
          lastCheck: formatLastCheck(svc),
        };
      }),
    [services]
  );

  const totalServices = SERVICE_NAMES.length;
  const onlineCount = useMemo(
    () => serviceCards.filter((c) => c.status.color === "success").length,
    [serviceCards]
  );

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox
          display="flex"
          justifyContent="space-between"
          alignItems="center"
          mb={3}
          flexWrap="wrap"
        >
          <VuiBox>
            <VuiTypography variant="h4" color="white" fontWeight="bold" mb={0.5}>
              服务健康详情
            </VuiTypography>
            <VuiTypography variant="body2" color="text">
              9 个核心服务实时健康状态
            </VuiTypography>
          </VuiBox>
          <Box
            sx={{
              display: "inline-flex",
              alignItems: "center",
              px: 2,
              py: 1,
              borderRadius: "12px",
              backgroundColor: "rgba(76,175,80,0.12)",
            }}
          >
            <Box
              sx={{
                width: 10,
                height: 10,
                borderRadius: "50%",
                backgroundColor: onlineCount === totalServices ? "#4CAF50" : onlineCount > 0 ? "#FF9800" : "#F44336",
                mr: 1,
              }}
            />
            <VuiTypography variant="button" color="white" fontWeight="bold">
              {isLoading ? "加载中..." : `${onlineCount}/${totalServices} 在线`}
            </VuiTypography>
          </Box>
        </VuiBox>

        {isLoading ? (
          <VuiBox display="flex" justifyContent="center" alignItems="center" minHeight="40vh">
            <CircularProgress color="info" />
          </VuiBox>
        ) : (
          <Grid container spacing={3}>
          {serviceCards.map(({ name, displayName, status, lastCheck }) => (
            <Grid item xs={12} md={6} xl={4} key={name}>
              <Card
                sx={{
                  p: 3,
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  background:
                    "linear-gradient(127deg, rgba(71,71,213,0.08) 0%, rgba(27,27,77,0.3) 100%)",
                  backdropFilter: "blur(8px)",
                  border: "1px solid rgba(255,255,255,0.04)",
                }}
              >
                <VuiBox display="flex" alignItems="center" mb={2}>
                  {/* Status dot */}
                  <Box
                    sx={{
                      width: 14,
                      height: 14,
                      minWidth: 14,
                      borderRadius: "50%",
                      backgroundColor: status.hex,
                      boxShadow: `0 0 10px ${status.hex}66`,
                      mr: 1.5,
                    }}
                  />
                  <VuiTypography variant="h6" color="white" fontWeight="bold">
                    {displayName}
                  </VuiTypography>
                </VuiBox>

                <VuiBox display="flex" justifyContent="space-between" alignItems="center">
                  <Tooltip title={`服务: ${name}`} placement="bottom">
                    <VuiBox
                      display="inline-flex"
                      px={2}
                      py={0.5}
                      borderRadius="lg"
                      bgColor={status.color}
                      color="white"
                      fontSize="12px"
                      fontWeight="bold"
                    >
                      {status.label}
                    </VuiBox>
                  </Tooltip>
                </VuiBox>

                <VuiBox mt="auto" pt={2}>
                  <VuiTypography variant="caption" color="text">
                    上次检测: {lastCheck}
                  </VuiTypography>
                </VuiBox>
              </Card>
            </Grid>
          ))}
        </Grid>
        )}
      </VuiBox>
    </DashboardLayout>
  );
}

export default SystemNodes;
