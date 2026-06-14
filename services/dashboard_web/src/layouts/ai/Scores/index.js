// OPT-306: AI 评分页 — Live + Shadow 评分实时展示
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card } from "@mui/material";

function AIScores() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <Card sx={{ p: 4, textAlign: "center" }}>
          <VuiTypography variant="h4" color="white" fontWeight="bold" mb={2}>
            AI 评分
          </VuiTypography>
          <VuiTypography variant="body1" color="text" mb={1}>
            Live AI 评分 + Shadow 评分 + 老化倒计时
          </VuiTypography>
          <VuiTypography variant="caption" color="text">
            OPT-306 — 待实现
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default AIScores;
