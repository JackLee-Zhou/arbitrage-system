// ============================================================================
// SidenavCard — Trading system status indicator
// ============================================================================

import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import { useVisionUIController } from "context";
import useSystemStore from "stores/system";

function SidenavCard({ ...rest }) {
  const [controller] = useVisionUIController();
  const { miniSidenav } = controller;
  const { systemMode, wsStatus } = useSystemStore();

  const modeLabel = systemMode === "normal" ? "正常" : systemMode === "safe" ? "安全模式" : systemMode === "paused" ? "已暂停" : systemMode;
  const modeColor = systemMode === "normal" ? "#00E676" : systemMode === "safe" ? "#FFAB00" : "#FF1744";

  return (
    <Card
      sx={{
        background: "rgba(0,229,255,0.05)",
        border: "1px solid rgba(0,229,255,0.10)",
        borderRadius: "10px",
        overflow: "hidden",
      }}
      {...rest}
    >
      <CardContent sx={{ p: 1.5, "&:last-child": { pb: 1.5 } }}>
        {/* Status row */}
        <VuiBox display="flex" alignItems="center" justifyContent="space-between" mb={1}>
          <VuiTypography variant="caption" color="text" fontWeight="medium" textTransform="uppercase" letterSpacing="0.5px">
            系统
          </VuiTypography>
          <VuiBox display="flex" alignItems="center" gap={0.5}>
            <VuiBox sx={{ width: 5, height: 5, borderRadius: "50%", bgcolor: modeColor, boxShadow: `0 0 5px ${modeColor}` }} />
            <VuiTypography variant="caption" color="white" fontWeight="medium" fontSize="11px">
              {modeLabel}
            </VuiTypography>
          </VuiBox>
        </VuiBox>

        {/* WS status */}
        <VuiBox display="flex" alignItems="center" justifyContent="space-between">
          <VuiTypography variant="caption" color="text" fontWeight="medium" textTransform="uppercase" letterSpacing="0.5px">
            WS
          </VuiTypography>
          <VuiBox display="flex" alignItems="center" gap={0.5}>
            <VuiBox
              sx={{
                width: 5,
                height: 5,
                borderRadius: "50%",
                bgcolor: wsStatus === "connected" ? "#00E676" : wsStatus === "connecting" ? "#FFAB00" : "#FF1744",
                boxShadow: `0 0 5px ${wsStatus === "connected" ? "#00E676" : wsStatus === "connecting" ? "#FFAB00" : "#FF1744"}`,
              }}
            />
            <VuiTypography variant="caption" color="white" fontWeight="medium" fontSize="11px">
              {wsStatus === "connected" ? "已连接" : wsStatus === "connecting" ? "连接中" : "离线"}
            </VuiTypography>
          </VuiBox>
        </VuiBox>
      </CardContent>
    </Card>
  );
}

export default SidenavCard;
