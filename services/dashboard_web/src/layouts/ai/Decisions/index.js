// OPT-311: 决策溯源 — P3 placeholder（全链路追溯就绪后接入）
import { useParams } from "react-router-dom";
import Card from "@mui/material/Card";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiBadge from "components/VuiBadge";
const AI_STALE = "#FFAB00";

const CHAIN_STEPS = [
  "raw", "parsed", "cleaned", "analysis", "snapshot", "score", "decision", "order",
];

function Decisions() {
  const { id } = useParams();

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox mb={3} display="flex" justifyContent="space-between" alignItems="center">
          <VuiTypography variant="h4" color="white" fontWeight="bold">
            🔍 决策溯源 — P3 阶段接入
          </VuiTypography>
          {id && (
            <VuiBadge color="info" size="sm">{id}</VuiBadge>
          )}
        </VuiBox>
        <Card sx={{ p: 3, mb: 3 }}>
          <VuiTypography variant="body1" color="white" fontWeight="medium" mb={2}>
            6 层 ID 反查（机制 6）:
          </VuiTypography>
          <VuiBox display="flex" gap={1} flexWrap="wrap" mb={3}>
            {CHAIN_STEPS.map((step, i) => (
              <VuiBadge key={step} color={i < 6 ? "info" : "dark"} variant="standard" size="sm">
                {i > 0 && <span style={{ marginRight: 4 }}>&rarr;</span>}
                {step}
              </VuiBadge>
            ))}
          </VuiBox>
          <VuiBox
            sx={{
              height: 200,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              borderRadius: 2,
              border: `1px dashed ${AI_STALE}44`,
              background: `repeating-linear-gradient(45deg, ${AI_STALE}08, ${AI_STALE}08 8px, transparent 8px, transparent 16px)`,
            }}
          >
            <VuiTypography variant="body2" color="text" textAlign="center" mb={1}>
              全链路追溯将在 P3 阶段实现
            </VuiTypography>
            <VuiTypography variant="caption" color="text">
              当前决策 ID: {id ?? "(无 — 从 AI 评分列表点击进入)"}
            </VuiTypography>
          </VuiBox>
        </Card>
        <Card sx={{ p: 2.5 }}>
          <VuiTypography variant="caption" color="text">
            溯源链路依赖 orchestrator API /api/decisions/:id/trace 端点
          </VuiTypography>
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default Decisions;
