// OPT-304: 期货委托页 — 当日委托列表
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function FuturesOrders() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            期货委托
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            当日委托列表（表格 + 筛选 + 分页）
          </VuiTypography>
          <VuiTypography variant="caption" color="text">
            OPT-304 — 待实现
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default FuturesOrders;
