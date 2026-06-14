// OPT-311: 舆情看板 — P3 placeholder（intel_collector + llm_scorer 就绪后接入）
import Card from "@mui/material/Card";
import Grid from "@mui/material/Grid";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { COLORS } from "theme/tokens";

const PLACEHOLDER_STYLES = {
  height: 160,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  borderRadius: 2,
  border: `1px dashed ${COLORS.AI_STALE}44`,
  background: `repeating-linear-gradient(45deg, ${COLORS.AI_STALE}08, ${COLORS.AI_STALE}08 8px, transparent 8px, transparent 16px)`,
};

function Sentiment() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox mb={3}>
          <VuiTypography variant="h4" color="white" fontWeight="bold">
            📊 舆情指数 — P3 阶段接入
          </VuiTypography>
        </VuiBox>
        <Card sx={{ p: 3, mb: 3 }}>
          <Grid container spacing={3}>
            <Grid item xs={12} md={4}>
              <VuiBox sx={{ ...PLACEHOLDER_STYLES }}>
                <VuiTypography variant="body2" color="text" textAlign="center">
                  舆情仪表盘 (Gauge)<br />P3 就绪
                </VuiTypography>
              </VuiBox>
            </Grid>
            <Grid item xs={12} md={8}>
              <VuiBox sx={{ ...PLACEHOLDER_STYLES, height: 300 }}>
                <VuiTypography variant="body2" color="text" textAlign="center">
                  新闻流 / 舆情时间线<br />P3 就绪
                </VuiTypography>
              </VuiBox>
            </Grid>
            <Grid item xs={12}>
              <VuiBox sx={{ ...PLACEHOLDER_STYLES }}>
                <VuiTypography variant="body2" color="text" textAlign="center">
                  舆情分布 (Positive / Neutral / Negative)<br />P3 就绪
                </VuiTypography>
              </VuiBox>
            </Grid>
          </Grid>
        </Card>
        <Card sx={{ p: 2.5 }}>
          <VuiTypography variant="caption" color="text">
            数据来源：intel_collector + llm_scorer 评分流水
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default Sentiment;
