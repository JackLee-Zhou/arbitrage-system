// ============================================================================
// App — arbitrage-system Dashboard Root
// Based on Vision UI Dashboard React (MIT License)
// ============================================================================

import { useState, useEffect, useMemo } from "react";

// react-router components
import { Route, Switch, Redirect, useLocation, useHistory } from "react-router-dom";

// @mui material components
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";

// Vision UI Dashboard React example components
import Sidenav from "examples/Sidenav";

// Vision UI Dashboard React themes
import theme from "assets/theme";

// Vision UI Dashboard React routes
import routes from "routes";

// Vision UI Dashboard React contexts
import { useVisionUIController, setMiniSidenav } from "context";

// App-wide providers
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "api/client";

// Mobile fallback
import MobileFallback from "pages/MobileFallback";

export default function App() {
  const [controller, dispatch] = useVisionUIController();
  const { miniSidenav, layout, sidenavColor } = controller;
  const [onMouseEnter, setOnMouseEnter] = useState(false);
  const { pathname } = useLocation();
  const history = useHistory();

  // ---- Detect mobile ----
  const isMobile = useMemo(() => {
    const width = window.innerWidth;
    const ua = navigator.userAgent;
    return width < 768 || /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(ua);
  }, []);

  const handleOnMouseEnter = () => {
    if (miniSidenav && !onMouseEnter) {
      setMiniSidenav(dispatch, false);
      setOnMouseEnter(true);
    }
  };

  const handleOnMouseLeave = () => {
    if (onMouseEnter) {
      setMiniSidenav(dispatch, true);
      setOnMouseEnter(false);
    }
  };

  useEffect(() => {
    document.documentElement.scrollTop = 0;
    document.scrollingElement.scrollTop = 0;
  }, [pathname]);

  useEffect(() => {
    if (isMobile && pathname !== "/mobile") {
      history.replace("/mobile");
    }
  }, [isMobile, pathname, history]);

  const getRoutes = (allRoutes) =>
    allRoutes.map((route) => {
      if (route.collapse) {
        return getRoutes(route.collapse);
      }
      if (route.route) {
        return <Route exact path={route.route} component={route.component} key={route.key} />;
      }
      return null;
    });

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        {layout === "dashboard" && (
          <Sidenav
            color={sidenavColor}
            brandName="Arbitrage"
            routes={routes}
            onMouseEnter={handleOnMouseEnter}
            onMouseLeave={handleOnMouseLeave}
          />
        )}
        <Switch>
          {getRoutes(routes)}
          <Route exact path="/mobile" component={MobileFallback} />
          <Redirect from="*" to="/" />
        </Switch>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
