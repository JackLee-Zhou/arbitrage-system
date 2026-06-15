// ============================================================================
// 主机性能 — CPU / Memory / Disk / Network — OPT-307
// ============================================================================

import { useQuery } from '@tanstack/react-query';
import Grid from "@mui/material/Grid";
import Card from "@mui/material/Card";
import CircularProgress from "@mui/material/CircularProgress";
import Box from "@mui/material/Box";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import { fetchOverview } from 'api/client';

import { BsCpu } from "react-icons/bs";
import { MdMemory } from "react-icons/md";
import { VscSymbolRuler } from "react-icons/vsc";
import { AiOutlineWifi } from "react-icons/ai";

function formatBytes(bytes) {
  if (bytes == null) return null;
  const units = ['B', 'KB', 'MB', 'GB'];
  let i = 0;
  let val = bytes;
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024;
    i++;
  }
  return `${val.toFixed(1)} ${units[i]}`;
}

const METRICS_DEF = [
  { key: "cpu", label: "CPU", icon: <BsCpu size={28} />, type: "percent" },
  { key: "memory", label: "Memory", icon: <MdMemory size={28} />, type: "percent" },
  { key: "disk", label: "Disk", icon: <VscSymbolRuler size={28} />, type: "percent" },
  { key: "network", label: "Network", icon: <AiOutlineWifi size={28} />, type: "throughput" },
];

function getMetricValue(host, key) {
  const m = host?.[key];
  if (!m) return null;
  if (typeof m.usage === 'number') return m.usage;
  if (typeof m === 'number') return m;
  return null;
}

function getMetricDetail(host, key) {
  const m = host?.[key];
  if (!m) return null;
  if (m.usage != null && m.total != null) {
    const used = typeof m.used === 'number' ? m.used : Math.round(m.usage * m.total / 100);
    return `${formatBytes(used)} / ${formatBytes(m.total)}`;
  }
  if (m.rx_bytes != null || m.tx_bytes != null) {
    const rx = m.rx_bytes ?? 0;
    const tx = m.tx_bytes ?? 0;
    return `RX ${formatBytes(rx)}/s  TX ${formatBytes(tx)}/s`;
  }
  if (m.detail) return m.detail;
  return null;
}

function thresholdColor(pct) {
  if (pct == null) return "info";
  if (pct > 80) return "error";
  if (pct > 60) return "warning";
  return "info";
}

function SystemHost() {
  const { data: overview, isLoading } = useQuery({
    queryKey: ['overview'],
    queryFn: fetchOverview,
    refetchInterval: 15000,
  });

  const host = overview?.host || {};

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox mb={3}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={0.5}>
            主机性能
          </VuiTypography>
          <VuiTypography variant="body2" color="text">
            CPU / Memory / Disk / Network 实时利用率
          </VuiTypography>
        </VuiBox>

        {isLoading ? (
          <VuiBox display="flex" justifyContent="center" alignItems="center" minHeight="40vh">
            <CircularProgress color="info" />
          </VuiBox>
        ) : (
          <Grid container spacing={3}>
            {METRICS_DEF.map(({ key, label, icon }) => {
              const value = getMetricValue(host, key);
              const pct = value != null ? Math.round(value) : 0;
              const displayPct = value != null ? `${pct}%` : '--%';
              const detail = getMetricDetail(host, key);

              return (
                <Grid item xs={12} md={6} key={key}>
                  <Card
                    sx={{
                      p: 3,
                      height: "100%",
                      display: "flex",
                      flexDirection: "column",
                      alignItems: "center",
                      justifyContent: "center",
                      background:
                        "linear-gradient(127deg, rgba(71,71,213,0.08) 0%, rgba(27,27,77,0.3) 100%)",
                      backdropFilter: "blur(8px)",
                      border: "1px solid rgba(255,255,255,0.04)",
                      minHeight: 300,
                    }}
                  >
                    <VuiBox color="info" mb={2} display="flex" alignItems="center">
                      {icon}
                    </VuiBox>

                    <VuiBox position="relative" display="inline-flex" mb={2}>
                      <CircularProgress
                        variant="determinate"
                        value={pct}
                        size={140}
                        thickness={4}
                        color={thresholdColor(value)}
                      />
                      <VuiBox
                        sx={{
                          top: 0,
                          left: 0,
                          bottom: 0,
                          right: 0,
                          position: "absolute",
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                        }}
                      >
                        <VuiTypography variant="h3" color="white" fontWeight="bold">
                          {displayPct}
                        </VuiTypography>
                      </VuiBox>
                    </VuiBox>

                    <VuiTypography variant="h5" color="white" fontWeight="bold" mb={0.5}>
                      {label}
                    </VuiTypography>

                    {detail ? (
                      <VuiTypography variant="caption" color="text" textAlign="center">
                        {detail}
                      </VuiTypography>
                    ) : (
                      <VuiTypography
                        variant="caption"
                        color="text"
                        textAlign="center"
                        sx={{ maxWidth: 280, lineHeight: 1.6 }}
                      >
                        {value != null ? '' : '暂无数据'}
                      </VuiTypography>
                    )}
                  </Card>
                </Grid>
              );
            })}
          </Grid>
        )}
      </VuiBox>
    </DashboardLayout>
  );
}

export default SystemHost;
