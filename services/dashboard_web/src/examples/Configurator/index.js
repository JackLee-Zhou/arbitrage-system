// ============================================================================
// Configurator — 主题配置面板（保留 Vision UI 主题微调能力）
// ============================================================================

import { useState, useEffect } from "react";

// @mui material components
import Divider from "@mui/material/Divider";
import IconButton from "@mui/material/IconButton";
import Icon from "@mui/material/Icon";

// Vision UI Dashboard React components
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiButton from "components/VuiButton";
import VuiSwitch from "components/VuiSwitch";

// Custom styles for the Configurator
import ConfiguratorRoot from "examples/Configurator/ConfiguratorRoot";

// Vision UI Dashboard React context
import {
  useVisionUIController,
  setOpenConfigurator,
  setTransparentSidenav,
  setFixedNavbar,
  setSidenavColor,
} from "context";

function Configurator() {
  const [controller, dispatch] = useVisionUIController();
  const { openConfigurator, transparentSidenav, fixedNavbar, sidenavColor } = controller;
  const [disabled, setDisabled] = useState(false);
  const sidenavColors = ["primary", "info", "success", "warning", "error"];

  useEffect(() => {
    function handleDisabled() {
      return window.innerWidth > 1200 ? setDisabled(false) : setDisabled(true);
    }
    window.addEventListener("resize", handleDisabled);
    handleDisabled();
    return () => window.removeEventListener("resize", handleDisabled);
  }, []);

  const handleCloseConfigurator = () => setOpenConfigurator(dispatch, false);
  const handleTransparentSidenav = () => setTransparentSidenav(dispatch, true);
  const handleWhiteSidenav = () => setTransparentSidenav(dispatch, false);
  const handleFixedNavbar = () => setFixedNavbar(dispatch, !fixedNavbar);

  const sidenavTypeButtonsStyles = ({
    functions: { pxToRem },
    boxShadows: { buttonBoxShadow },
  }) => ({
    height: pxToRem(42),
    boxShadow: buttonBoxShadow.main,
    "&:hover, &:focus": { opacity: 1 },
  });

  return (
    <ConfiguratorRoot variant="permanent" ownerState={{ openConfigurator }}>
      <VuiBox
        backgroundColor="black"
        display="flex"
        justifyContent="space-between"
        alignItems="baseline"
        pt={3}
        pb={0.8}
        px={3}
      >
        <VuiBox>
          <VuiTypography color="white" variant="h5" fontWeight="bold">
            主题配置
          </VuiTypography>
          <VuiTypography variant="body2" color="white" fontWeight="bold">
            侧边栏颜色 / 类型 / 导航栏设置
          </VuiTypography>
        </VuiBox>

        <Icon
          sx={({ typography: { size, fontWeightBold }, palette: { white } }) => ({
            fontSize: `${size.md} !important`,
            fontWeight: `${fontWeightBold} !important`,
            stroke: `${white.main} !important`,
            strokeWidth: "2px",
            cursor: "pointer",
            mt: 2,
          })}
          onClick={handleCloseConfigurator}
        >
          close
        </Icon>
      </VuiBox>

      <Divider light />

      <VuiBox pt={1.25} pb={3} px={3}>
        {/* ---- Sidenav Colors ---- */}
        <VuiBox>
          <VuiTypography variant="h6" color="white">
            侧边栏颜色
          </VuiTypography>
          <VuiBox mb={0.5}>
            {sidenavColors.map((color) => (
              <IconButton
                key={color}
                sx={({ borders: { borderWidth }, palette: { white, dark }, transitions }) => ({
                  width: "24px",
                  height: "24px",
                  padding: 0,
                  border: `${borderWidth[1]} solid ${white.main}`,
                  borderColor: sidenavColor === color && dark.main,
                  transition: transitions.create("border-color", {
                    easing: transitions.easing.sharp,
                    duration: transitions.duration.shorter,
                  }),
                  backgroundImage: ({ functions: { linearGradient }, palette: { gradients } }) =>
                    linearGradient(gradients[color].main, gradients[color].state),
                  "&:not(:last-child)": { mr: 1 },
                  "&:hover, &:focus, &:active": { borderColor: dark.main },
                })}
                onClick={() => setSidenavColor(dispatch, color)}
              />
            ))}
          </VuiBox>
        </VuiBox>

        {/* ---- Sidenav Type ---- */}
        {window.innerWidth >= 1440 && (
          <VuiBox mt={3} lineHeight={1}>
            <VuiTypography variant="h6" color="white">
              侧边栏类型
            </VuiTypography>
            <VuiTypography variant="button" color="text" fontWeight="regular">
              透明 / 不透明
            </VuiTypography>
            <VuiBox sx={{ display: "flex", mt: 2 }}>
              <VuiButton
                color="info"
                variant={transparentSidenav ? "contained" : "outlined"}
                onClick={handleTransparentSidenav}
                disabled={disabled}
                fullWidth
                sx={{ mr: 1, ...sidenavTypeButtonsStyles }}
              >
                透明
              </VuiButton>
              <VuiButton
                color="info"
                variant={transparentSidenav ? "outlined" : "contained"}
                onClick={handleWhiteSidenav}
                disabled={disabled}
                fullWidth
                sx={sidenavTypeButtonsStyles}
              >
                不透明
              </VuiButton>
            </VuiBox>
          </VuiBox>
        )}

        {/* ---- Navbar Fixed ---- */}
        <VuiBox mt={3} mb={2} lineHeight={1}>
          <VuiTypography variant="h6" color="white">
            固定导航栏
          </VuiTypography>
          <VuiSwitch checked={fixedNavbar} onChange={handleFixedNavbar} color="info" />
        </VuiBox>
      </VuiBox>
    </ConfiguratorRoot>
  );
}

export default Configurator;
