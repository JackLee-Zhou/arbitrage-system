// ============================================================================
// 风控参数（V1 只读） — OPT-308
// Display cards: 单笔最大仓位, 日最大亏损, 紧急阈值, 合约乘数
// ============================================================================

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card, Grid } from "@mui/material";
import { IoInformationCircle } from "react-icons/io5";
import { IoShieldCheckmark } from "react-icons/io5";
import { IoWarning } from "react-icons/io5";
import { IoFlash } from "react-icons/io5";
import { IoCalculator } from "react-icons/io5";

const RISK_PARAMS = [
  {
    label: "单笔最大仓位",
    value: "3 手",
    sub: "单笔开仓不超过 3 手 MA 合约",
    icon: <IoShieldCheckmark size="28px" color="#01b574" />,
  },
  {
    label: "日最大亏损",
    value: "¥ 5,000",
    sub: "单日累计亏损超过该值自动停止所有策略",
    icon: <IoWarning size="28px" color="#e31a1a" />,
  },
  {
    label: "紧急阈值",
    value: "¥ 10,000",
    sub: "日亏损达到该值触发紧急平仓 + 通知",
    icon: <IoFlash size="28px" color="#ffb547" />,
  },
  {
    label: "合约乘数",
    value: "10 元/点",
    sub: "MA 主力合约每点价值 10 元人民币",
    icon: <IoCalculator size="28px" color="#0075ff" />,
  },
];

function ManageRisk() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        {/* ---- 标题 ---- */}
        <VuiTypography variant="h4" color="white" fontWeight="bold" mb={1}>
          风控参数
        </VuiTypography>

        {/* ---- V1 只读提示 ---- */}
        <Card sx={{ p: 2, mb: 3, display: "flex", alignItems: "center", gap: 1.5 }}>
          <IoInformationCircle size="22px" color="#0075ff" />
          <VuiTypography variant="body2" color="text">
            V1 只读模式 — 风控参数修改功能将在 P3 阶段开放
          </VuiTypography>
        </Card>

        {/* ---- 风控参数卡片 ---- */}
        <Grid container spacing={3}>
          {RISK_PARAMS.map((param) => (
            <Grid item xs={12} md={6} xl={3} key={param.label}>
              <Card sx={{ p: 3, height: "100%" }}>
                <VuiBox display="flex" alignItems="center" mb={2}>
                  <VuiBox
                    display="flex"
                    alignItems="center"
                    justifyContent="center"
                    width="48px"
                    height="48px"
                    borderRadius="12px"
                    bgColor="info"
                    mr={2}
                    sx={{ opacity: 0.85 }}
                  >
                    {param.icon}
                  </VuiBox>
                  <VuiTypography variant="h6" color="text" fontWeight="regular">
                    {param.label}
                  </VuiTypography>
                </VuiBox>
                <VuiTypography variant="h3" color="white" fontWeight="bold" mb={1}>
                  {param.value}
                </VuiTypography>
                <VuiTypography variant="caption" color="text">
                  {param.sub}
                </VuiTypography>
              </Card>
            </Grid>
          ))}
        </Grid>
      </VuiBox>
    </DashboardLayout>
  );
}

export default ManageRisk;
