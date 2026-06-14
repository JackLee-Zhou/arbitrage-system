// OPT-304: 期货成交页 — 当日成交明细
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function FuturesTrades() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            期货成交
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            当日成交明细
          </VuiTypography>
          <VuiTypography variant="caption" color="text">
            OPT-304 — 待实现
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default FuturesTrades;
