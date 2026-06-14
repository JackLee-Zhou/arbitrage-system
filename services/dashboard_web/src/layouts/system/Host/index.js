// ============================================================================
// 主机性能 — CPU / Memory / Disk / Network — OPT-307
// ============================================================================

import Grid from "@mui/material/Grid";
import Card from "@mui/material/Card";
import CircularProgress from "@mui/material/CircularProgress";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import { BsCpu } from "react-icons/bs";
import { MdMemory } from "react-icons/md";
import { VscSymbolRuler } from "react-icons/vsc";
import { AiOutlineWifi } from "react-icons/ai";

const METRICS = [
  { key: "cpu", label: "CPU", icon: <BsCpu size={28} /> },
  { key: "memory", label: "Memory", icon: <MdMemory size={28} /> },
  { key: "disk", label: "Disk", icon: <VscSymbolRuler size={28} /> },
  { key: "network", label: "Network", icon: <AiOutlineWifi size={28} /> },
];

function SystemHost() {
  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiTypography variant="h4" color="white" fontWeight="bold" mb={0.5}>
          主机性能
        </VuiTypography>
        <VuiTypography variant="body2" color="text" mb={3}>
          CPU / Memory / Disk / Network 实时利用率
        </VuiTypography>

        <Grid container spacing={3}>
          {METRICS.map(({ key, label, icon }) => (
            <Grid item xs={12} md={6} key={key}>
              <Card
                sx={{
                  p: 3,
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  background:
                    "linear-gradient(127deg, rgba(71,71,213,0.08) 0%, rgba(27,27,77,0.3) 100%)",
                  backdropFilter: "blur(8px)",
                  border: "1px solid rgba(255,255,255,0.04)",
                  minHeight: 300,
                }}
              >
                {/* Icon */}
                <VuiBox color="info" mb={2} display="flex" alignItems="center">
                  {icon}
                </VuiBox>

                {/* CircularProgress placeholder */}
                <VuiBox position="relative" display="inline-flex" mb={2}>
                  <CircularProgress
                    variant="determinate"
                    value={0}
                    size={140}
                    thickness={4}
                    color="info"
                  />
                  <VuiBox
                    sx={{
                      top: 0,
                      left: 0,
                      bottom: 0,
                      right: 0,
                      position: "absolute",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                    }}
                  >
                    <VuiTypography variant="h3" color="white" fontWeight="bold">
                      --%
                    </VuiTypography>
                  </VuiBox>
                </VuiBox>

                {/* Label */}
                <VuiTypography variant="h5" color="white" fontWeight="bold" mb={0.5}>
                  {label}
                </VuiTypography>

                {/* Placeholder note */}
                <VuiTypography
                  variant="caption"
                  color="text"
                  textAlign="center"
                  sx={{ maxWidth: 280, lineHeight: 1.6 }}
                >
                  主机性能数据将在 orchestrator API 就绪后接入
                </VuiTypography>
              </Card>
            </Grid>
          ))}
        </Grid>
      </VuiBox>
    </DashboardLayout>
  );
}

export default SystemHost;
