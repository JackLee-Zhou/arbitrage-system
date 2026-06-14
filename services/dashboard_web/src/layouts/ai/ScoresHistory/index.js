// OPT-306: AI 评分历史页 — Tremor LineChart 曲线
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function AIScoresHistory() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            AI 评分历史
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            评分历史曲线（Tremor LineChart）
          </VuiTypography>
          <VuiTypography variant="caption" color="text">
            OPT-306 — 待实现
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default AIScoresHistory;
