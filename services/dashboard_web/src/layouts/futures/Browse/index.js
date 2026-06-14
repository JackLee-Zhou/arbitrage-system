// OPT-303: 期货合约浏览页 — 左列表 + 右详情大卡片
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function FuturesBrowse() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            期货合约浏览
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            左列表 + 右详情大卡片（分时/日K/盘口五档/持仓）
          </VuiTypography>
          <VuiTypography variant="caption" color="text">
            OPT-303 — 待实现
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default FuturesBrowse;
