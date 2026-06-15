// ============================================================================
// OPT-310: 策略管理 + Monaco Editor placeholder
// Tabs: "策略列表" | "新建策略"
// ============================================================================

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { fetchStrategies } from "api/client";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiButton from "components/VuiButton";
import Table from "examples/Tables/Table";
import { Card, Tabs, Tab, Switch, CircularProgress } from "@mui/material";

const columns = [
  { name: "name", align: "left" },
  { name: "symbol", align: "center" },
  { name: "type", align: "center" },
  { name: "status", align: "center" },
  { name: "enabled", align: "center" },
  { name: "created", align: "center" },
  { name: "return", align: "right" },
];

const StatusBadge = ({ s }) => {
  const live = s === "LIVE";
  return (
    <VuiBox display="inline-flex" alignItems="center" px={1.5} py={0.5}
      borderRadius="12px" bgColor={live ? "#01b57420" : "#ffb54720"}
      sx={{ border: `1px solid ${live ? "#01b574" : "#ffb547"}` }}
    >
      <VuiBox width="6px" height="6px" borderRadius="50%" bgColor={live ? "success" : "warning"} mr={0.5} />
      <VuiTypography variant="caption" color={live ? "success" : "warning"} fontWeight="bold">{s}</VuiTypography>
    </VuiBox>
  );
};

const ReturnCell = ({ v }) => {
  const color = v.startsWith("+") ? "success" : v.startsWith("-") ? "error" : "text";
  return <VuiTypography variant="button" color={color} fontWeight="bold">{v}</VuiTypography>;
};

const ToggleBtn = () => <Switch disabled size="small" defaultChecked color="success" />;

function strategyToRow(s) {
  return {
    name: s.name || "-",
    symbol: s.symbol || "-",
    type: s.type || "-",
    status: <StatusBadge s={s.status || "UNKNOWN"} />,
    enabled: <ToggleBtn />,
    created: s.created_at ? s.created_at.slice(0, 10) : "-",
    return: <ReturnCell v={s.pnl || "0.00%"} />,
  };
}

function ManageStrategies() {
  const [tab, setTab] = useState(0);
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["strategies"],
    queryFn: fetchStrategies,
  });

  const strategies = data?.strategies || [];
  const rows = strategies.map(strategyToRow);

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiTypography variant="h4" color="white" fontWeight="bold" mb={1}>策略管理</VuiTypography>
        <Card sx={{ mb: 3 }}>
          <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ px: 2, pt: 1 }}>
            <Tab label="策略列表" />
            <Tab label="新建策略" />
          </Tabs>
        </Card>
        {tab === 0 ? (
          <Card sx={{ overflow: "hidden" }}>
            {/* ---- Loading ---- */}
            {isLoading && (
              <VuiBox display="flex" justifyContent="center" alignItems="center" minHeight={200}>
                <CircularProgress color="info" />
              </VuiBox>
            )}

            {/* ---- Error ---- */}
            {isError && (
              <VuiBox display="flex" flexDirection="column" alignItems="center" justifyContent="center" minHeight={200} px={3}>
                <VuiTypography variant="body2" color="error" mb={2}>
                  加载策略列表失败：{error?.message || "未知错误"}
                </VuiTypography>
                <VuiButton variant="outlined" color="info" size="small" onClick={() => window.location.reload()}>
                  重试
                </VuiButton>
              </VuiBox>
            )}

            {/* ---- Empty ---- */}
            {!isLoading && !isError && strategies.length === 0 && (
              <VuiBox display="flex" alignItems="center" justifyContent="center" minHeight={200}>
                <VuiTypography variant="body1" color="text">暂无策略数据</VuiTypography>
              </VuiBox>
            )}

            {/* ---- Table ---- */}
            {!isLoading && !isError && strategies.length > 0 && (
              <VuiBox sx={{
                "& th": { borderBottom: ({ borders: { borderWidth }, palette: { grey } }) => `${borderWidth[1]} solid ${grey[700]}` },
                "& .MuiTableRow-root:not(:last-child) td": { borderBottom: ({ borders: { borderWidth }, palette: { grey } }) => `${borderWidth[1]} solid ${grey[700]}` },
              }}>
                <Table columns={columns} rows={rows} />
              </VuiBox>
            )}
          </Card>
        ) : (
          <Card sx={{ p: 3, textAlign: "center", display: "flex", alignItems: "center", justifyContent: "center", minHeight: 200 }}>
            <VuiTypography variant="body1" color="text">Monaco YAML 编辑器将在 P3 阶段集成</VuiTypography>
          </Card>
        )}
      </VuiBox>
    </DashboardLayout>
  );
}

export default ManageStrategies;
