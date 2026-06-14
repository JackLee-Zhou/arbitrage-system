// ============================================================================
// 数据源列表（V1 只读） — OPT-308
// 展示：domestic_sources.yaml, overseas_sources.yaml, social_sources.yaml
// ============================================================================

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { Card, Grid } from "@mui/material";
import { IoInformationCircle } from "react-icons/io5";
import { IoDocumentText } from "react-icons/io5";
import { IoGlobe } from "react-icons/io5";
import { IoChatbubbles } from "react-icons/io5";

const SOURCES = [
  {
    name: "domestic_sources.yaml",
    description: "国内期货市场数据源（上期所、郑商所、大商所、中金所）",
    icon: <IoDocumentText size="24px" color="#0075ff" />,
    fields: [
      { label: "路径", value: "config/sources/domestic_sources.yaml" },
      { label: "类型", value: "YAML 配置" },
      { label: "状态", value: "生效中" },
    ],
  },
  {
    name: "overseas_sources.yaml",
    description: "海外市场数据源（CME、ICE、LME 等）",
    icon: <IoGlobe size="24px" color="#01b574" />,
    fields: [
      { label: "路径", value: "config/sources/overseas_sources.yaml" },
      { label: "类型", value: "YAML 配置" },
      { label: "状态", value: "生效中" },
    ],
  },
  {
    name: "social_sources.yaml",
    description: "舆情数据源（新闻、社交媒体、研报）",
    icon: <IoChatbubbles size="24px" color="#ffb547" />,
    fields: [
      { label: "路径", value: "config/sources/social_sources.yaml" },
      { label: "类型", value: "YAML 配置" },
      { label: "状态", value: "生效中" },
    ],
  },
];

function ManageSources() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        {/* ---- 标题 ---- */}
        <VuiTypography variant="h4" color="white" fontWeight="bold" mb={1}>
          数据源管理
        </VuiTypography>

        {/* ---- V1 只读提示 ---- */}
        <Card sx={{ p: 2, mb: 3, display: "flex", alignItems: "center", gap: 1.5 }}>
          <IoInformationCircle size="22px" color="#0075ff" />
          <VuiTypography variant="body2" color="text">
            数据源管理 — V1 只读。编辑功能将在 P3 阶段开放
          </VuiTypography>
        </Card>

        {/* ---- 数据源列表 ---- */}
        <Grid container spacing={3}>
          {SOURCES.map((source) => (
            <Grid item xs={12} key={source.name}>
              <Card sx={{ p: 3 }}>
                <VuiBox display="flex" alignItems="flex-start" gap={2}>
                  {/* 左侧图标 */}
                  <VuiBox
                    display="flex"
                    alignItems="center"
                    justifyContent="center"
                    width="44px"
                    height="44px"
                    borderRadius="10px"
                    bgColor="info"
                    flexShrink={0}
                    sx={{ opacity: 0.85 }}
                  >
                    {source.icon}
                  </VuiBox>

                  {/* 右侧内容 */}
                  <VuiBox flex={1}>
                    <VuiTypography variant="h6" color="white" fontWeight="bold" mb={0.5}>
                      {source.name}
                    </VuiTypography>
                    <VuiTypography variant="body2" color="text" mb={2}>
                      {source.description}
                    </VuiTypography>

                    <VuiBox
                      display="grid"
                      sx={{
                        gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
                        gap: 1.5,
                      }}
                    >
                      {source.fields.map((field) => (
                        <VuiBox key={field.label}>
                          <VuiTypography variant="caption" color="text" display="block" mb={0.25}>
                            {field.label}
                          </VuiTypography>
                          <VuiTypography variant="button" color="white" fontWeight="medium">
                            {field.value}
                          </VuiTypography>
                        </VuiBox>
                      ))}
                    </VuiBox>
                  </VuiBox>
                </VuiBox>
              </Card>
            </Grid>
          ))}
        </Grid>
      </VuiBox>
    </DashboardLayout>
  );
}

export default ManageSources;
