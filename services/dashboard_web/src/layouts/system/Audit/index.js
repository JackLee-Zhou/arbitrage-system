// ============================================================================
// 审计日志列表（最近 100 条） — OPT-307
// ============================================================================

import { useMemo } from "react";
import Card from "@mui/material/Card";
import Box from "@mui/material/Box";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import Table from "examples/Tables/Table";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import useSystemStore from "stores/system";

function formatTime(ts) {
  if (!ts) return "--";
  try {
    const d = new Date(ts);
    if (isNaN(d.getTime())) return "--";
    const pad = (n) => String(n).padStart(2, "0");
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  } catch {
    return "--";
  }
}

function getAlertField(alert, ...fields) {
  for (const f of fields) {
    const v = alert[f];
    if (v != null && v !== "") return String(v);
  }
  return "--";
}

const COLUMNS = [
  { name: "time", align: "left", width: "160px" },
  { name: "operation", align: "left" },
  { name: "operator", align: "left", width: "120px" },
  { name: "detail", align: "left" },
];

function SystemAudit() {
  const { alerts } = useSystemStore();

  const rows = useMemo(
    () =>
      alerts.map((alert) => ({
        time: formatTime(alert.timestamp),
        operation: getAlertField(alert, "operation", "action", "message", "type"),
        operator: getAlertField(alert, "operator", "user", "source"),
        detail: getAlertField(alert, "details", "detail", "description", "content"),
      })),
    [alerts]
  );

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox mb={3}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={0.5}>
            审计日志
          </VuiTypography>
          <VuiTypography variant="body2" color="text">
            最近 100 条操作记录
          </VuiTypography>
        </VuiBox>

        <Card
          sx={{
            p: 2,
            background:
              "linear-gradient(127deg, rgba(71,71,213,0.08) 0%, rgba(27,27,77,0.3) 100%)",
            backdropFilter: "blur(8px)",
            border: "1px solid rgba(255,255,255,0.04)",
          }}
        >
          {rows.length > 0 ? (
            <Table columns={COLUMNS} rows={rows} />
          ) : (
            <Box
              sx={{
                display: "flex",
                justifyContent: "center",
                alignItems: "center",
                py: 8,
              }}
            >
              <VuiTypography variant="body1" color="text">
                暂无审计记录
              </VuiTypography>
            </Box>
          )}
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default SystemAudit;
