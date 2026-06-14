// OPT-305: 股票占位页 — V2 实施前提示
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function StocksPlaceholder() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h2" mb={2}>
            📉
          </VuiTypography>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            股票模块
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            状态：规划中（V2 阶段实施）
          </VuiTypography>
          <VuiTypography variant="body2" color="text" mb={1}>
            计划接入：QMT / easytrader
          </VuiTypography>
          <VuiTypography variant="body2" color="text">
            预计上线：P3 完成后
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default StocksPlaceholder;
