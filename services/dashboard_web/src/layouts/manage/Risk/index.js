// ============================================================================
// 风控参数（V1 只读） — OPT-308
// Display cards: 单笔最大仓位, 日最大亏损, 紧急阈值, 合约乘数
// ============================================================================

import { useQuery } from "@tanstack/react-query";
import { fetchRiskParams } from "api/client";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiButton from "components/VuiButton";
import { Card, Grid, CircularProgress } from "@mui/material";
import { IoInformationCircle } from "react-icons/io5";
import { IoShieldCheckmark } from "react-icons/io5";
import { IoWarning } from "react-icons/io5";
import { IoFlash } from "react-icons/io5";
import { IoCalculator } from "react-icons/io5";

// 风控参数卡片元数据（标签 + 图标 + 后备子文本）
const PARAM_META = {
  max_position_size: {
    label: "单笔最大仓位",
    icon: <IoShieldCheckmark size="28px" color="#01b574" />,
    defaultSub: "单笔开仓不超过 N 手 MA 合约",
  },
  daily_loss_limit: {
    label: "日最大亏损",
    icon: <IoWarning size="28px" color="#e31a1a" />,
    defaultSub: "单日累计亏损超过该值自动停止所有策略",
  },
  emergency_threshold: {
    label: "紧急阈值",
    icon: <IoFlash size="28px" color="#ffb547" />,
    defaultSub: "日亏损达到该值触发紧急平仓 + 通知",
  },
  contract_multiplier: {
    label: "合约乘数",
    icon: <IoCalculator size="28px" color="#0075ff" />,
    defaultSub: "MA 主力合约每点价值 N 元人民币",
  },
};

function RiskParamCard({ paramKey, value, sub }) {
  const meta = PARAM_META[paramKey];
  if (!meta) return null;

  return (
    <Grid item xs={12} md={6} xl={3}>
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
            {meta.icon}
          </VuiBox>
          <VuiTypography variant="h6" color="text" fontWeight="regular">
            {meta.label}
          </VuiTypography>
        </VuiBox>
        <VuiTypography variant="h3" color="white" fontWeight="bold" mb={1}>
          {value}
        </VuiTypography>
        <VuiTypography variant="caption" color="text">
          {sub}
        </VuiTypography>
      </Card>
    </Grid>
  );
}

function ManageRisk() {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["risk-params"],
    queryFn: fetchRiskParams,
  });

  const riskParams = data?.risk_params || {};
  const paramKeys = Object.keys(riskParams);
  const hasData = paramKeys.length > 0;

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

        {/* ---- Loading ---- */}
        {isLoading && (
          <VuiBox display="flex" justifyContent="center" alignItems="center" minHeight={200}>
            <CircularProgress color="info" />
          </VuiBox>
        )}

        {/* ---- Error ---- */}
        {isError && (
          <VuiBox display="flex" flexDirection="column" alignItems="center" justifyContent="center" minHeight={200}>
            <VuiTypography variant="body2" color="error" mb={2}>
              加载风控参数失败：{error?.message || "未知错误"}
            </VuiTypography>
            <VuiButton variant="outlined" color="info" size="small" onClick={() => window.location.reload()}>
              重试
            </VuiButton>
          </VuiBox>
        )}

        {/* ---- Empty ---- */}
        {!isLoading && !isError && !hasData && (
          <VuiBox display="flex" alignItems="center" justifyContent="center" minHeight={200}>
            <VuiTypography variant="body1" color="text">暂无风控参数数据</VuiTypography>
          </VuiBox>
        )}

        {/* ---- 风控参数卡片 ---- */}
        {!isLoading && !isError && hasData && (
          <Grid container spacing={3}>
            {paramKeys.map((key) => {
              const param = riskParams[key] || {};
              const value = typeof param === "object" ? param.value ?? String(param) : String(param);
              const sub = param.description || PARAM_META[key]?.defaultSub || "";
              return <RiskParamCard key={key} paramKey={key} value={value} sub={sub} />;
            })}
          </Grid>
        )}
      </VuiBox>
    </DashboardLayout>
  );
}

export default ManageRisk;
