// ============================================================================
// DashboardNavbar — 顶部导航栏（保留 Vision UI 风格）
// 包含 futures 页面右侧持仓固定栏
// ============================================================================

import { useState, useEffect } from "react";
import { useLocation } from "react-router-dom";
import PropTypes from "prop-types";

import AppBar from "@mui/material/AppBar";
import Toolbar from "@mui/material/Toolbar";
import IconButton from "@mui/material/IconButton";
import Icon from "@mui/material/Icon";

import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import {
  navbar,
  navbarContainer,
  navbarRow,
  navbarIconButton,
  navbarMobileMenu,
} from "examples/Navbars/DashboardNavbar/styles";

import {
  useVisionUIController,
  setTransparentNavbar,
  setMiniSidenav,
  setOpenConfigurator,
} from "context";

import useTradingStore from "stores/trading";

function DashboardNavbar({ absolute, light, isMini }) {
  const [navbarType, setNavbarType] = useState();
  const [controller, dispatch] = useVisionUIController();
  const { miniSidenav, transparentNavbar, fixedNavbar } = controller;
  const location = useLocation();
  const route = location.pathname.split("/").slice(1);

  // ---- 持仓栏数据 ----
  const { positions, totalFloatPnL } = useTradingStore();
  const isFuturesRoute = location.pathname.startsWith("/futures/");

  // 构建持仓栏内容
  const positionKeys = Object.keys(positions).filter(
    (k) => positions[k] && positions[k].symbol
  );
  const positionCount = positionKeys.length;

  let positionText = "";
  let positionPnl = 0;
  let positionClickable = false;

  if (isFuturesRoute) {
    if (positionCount === 0) {
      positionText = "💰 当前无持仓";
      positionPnl = 0;
    } else if (positionCount === 1) {
      const pos = positions[positionKeys[0]];
      const qty = pos.quantity || pos.qty || 0;
      const pnl = pos.floatPnL || 0;
      positionPnl = pnl;
      positionText = `💰 持仓: ${pos.symbol} ${qty > 0 ? "+" : ""}${qty} ¥${pnl >= 0 ? "+" : ""}${pnl.toFixed(0)}`;
    } else {
      positionPnl = totalFloatPnL || 0;
      positionText = `💰 总持仓 ${positionCount} 个 ¥${positionPnl >= 0 ? "+" : ""}${positionPnl.toFixed(0)}`;
      positionClickable = true;
    }
  }

  useEffect(() => {
    if (fixedNavbar) {
      setNavbarType("sticky");
    } else {
      setNavbarType("static");
    }

    function handleTransparentNavbar() {
      setTransparentNavbar(dispatch, (fixedNavbar && window.scrollY === 0) || !fixedNavbar);
    }

    window.addEventListener("scroll", handleTransparentNavbar);
    handleTransparentNavbar();
    return () => window.removeEventListener("scroll", handleTransparentNavbar);
  }, [dispatch, fixedNavbar]);

  const handleMiniSidenav = () => setMiniSidenav(dispatch, !miniSidenav);
  const handleConfiguratorOpen = () => setOpenConfigurator(dispatch, true);

  // Route breadcrumb text
  const routeText = route[route.length - 1] || "概览";

  return (
    <AppBar
      position={absolute ? "absolute" : navbarType}
      color="inherit"
      sx={(theme) => navbar(theme, { transparentNavbar, absolute, light })}
    >
      <Toolbar sx={(theme) => navbarContainer(theme)}>
        <VuiBox color="inherit" mb={{ xs: 1, md: 0 }} sx={(theme) => navbarRow(theme, { isMini })}>
          <VuiTypography
            variant="h6"
            color={light ? "white" : "dark"}
            fontWeight="bold"
            textTransform="capitalize"
          >
            {routeText}
          </VuiTypography>
        </VuiBox>
        {isMini ? null : (
          <VuiBox sx={(theme) => navbarRow(theme, { isMini })}>
            {/* ---- 持仓固定栏（仅 /futures/* 可见） ---- */}
            {isFuturesRoute && (
              <VuiBox
                display="flex"
                alignItems="center"
                mr={2}
                px={1.5}
                py={0.5}
                borderRadius="lg"
                bgColor="info"
                sx={{ opacity: 0.85 }}
              >
                <VuiTypography
                  variant="button"
                  color={positionPnl > 0 ? "success" : positionPnl < 0 ? "error" : "white"}
                  fontWeight="bold"
                  sx={{
                    cursor: positionClickable ? "pointer" : "default",
                    whiteSpace: "nowrap",
                  }}
                >
                  {positionText}
                </VuiTypography>
              </VuiBox>
            )}
            <VuiBox color={light ? "white" : "inherit"}>
              <IconButton
                size="small"
                color="inherit"
                sx={navbarMobileMenu}
                onClick={handleMiniSidenav}
              >
                <Icon className={"text-white"}>{miniSidenav ? "menu_open" : "menu"}</Icon>
              </IconButton>
              <IconButton
                size="small"
                color="inherit"
                sx={navbarIconButton}
                onClick={handleConfiguratorOpen}
              >
                <Icon>settings</Icon>
              </IconButton>
            </VuiBox>
          </VuiBox>
        )}
      </Toolbar>
    </AppBar>
  );
}

DashboardNavbar.defaultProps = {
  absolute: false,
  light: false,
  isMini: false,
};

DashboardNavbar.propTypes = {
  absolute: PropTypes.bool,
  light: PropTypes.bool,
  isMini: PropTypes.bool,
};

export default DashboardNavbar;
