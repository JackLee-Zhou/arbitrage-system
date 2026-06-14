// ============================================================================
// 审计日志列表（最近 100 条） — OPT-307
// ============================================================================

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function SystemAudit() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            SystemAudit
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            审计日志列表（最近 100 条）
          </VuiTypography>
          <VuiTypography variant="caption" color="text">
            OPT: OPT-307 — 待实现
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default SystemAudit;
