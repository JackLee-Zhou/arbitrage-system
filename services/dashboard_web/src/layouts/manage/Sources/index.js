// ============================================================================
// 数据源列表（V1 只读） — OPT-308
// ============================================================================

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function ManageSources() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            ManageSources
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            数据源列表（V1 只读）
          </VuiTypography>
          <VuiTypography variant="caption" color="text">
            OPT: OPT-308 — 待实现
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default ManageSources;
