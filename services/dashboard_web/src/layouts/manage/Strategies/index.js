// ============================================================================
// OPT-310: 策略管理 + Monaco Editor placeholder
// Tabs: "策略列表" | "新建策略"
// ============================================================================

import { useState } from "react";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import Table from "examples/Tables/Table";
import { Card, Tabs, Tab, Switch } from "@mui/material";

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

const rows = [
  { name: "MA 趋势追踪 v1", symbol: "MA509", type: "趋势跟踪", status: <StatusBadge s="LIVE" />, enabled: <ToggleBtn />, created: "2026-03-15", return: <ReturnCell v="+8.32%" /> },
  { name: "MA 均线交叉 v2", symbol: "MA509", type: "均值回归", status: <StatusBadge s="SHADOW" />, enabled: <ToggleBtn />, created: "2026-04-01", return: <ReturnCell v="+2.15%" /> },
  { name: "跨期套利 v1", symbol: "MA509/MA601", type: "套利", status: <StatusBadge s="SHADOW" />, enabled: <ToggleBtn />, created: "2026-05-10", return: <ReturnCell v="-1.47%" /> },
];

function ManageStrategies() {
  const [tab, setTab] = useState(0);
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
            <VuiBox sx={{
              "& th": { borderBottom: ({ borders: { borderWidth }, palette: { grey } }) => `${borderWidth[1]} solid ${grey[700]}` },
              "& .MuiTableRow-root:not(:last-child) td": { borderBottom: ({ borders: { borderWidth }, palette: { grey } }) => `${borderWidth[1]} solid ${grey[700]}` },
            }}>
              <Table columns={columns} rows={rows} />
            </VuiBox>
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
