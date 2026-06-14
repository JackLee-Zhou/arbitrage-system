function collapseItem(theme, ownerState) {
  const { palette, transitions, breakpoints, functions } = theme;
  const { active, transparentSidenav } = ownerState;

  const { transparent, dark } = palette;
  const { pxToRem } = functions;

  return {
    background: active ? "rgba(0,117,255,0.18)" : transparent.main,
    color: active ? palette.white.main : palette.text.main,
    display: "flex",
    alignItems: "center",
    width: "100%",
    padding: `${pxToRem(10)} ${pxToRem(14)} ${pxToRem(10)} ${pxToRem(14)}`,
    margin: `${pxToRem(1)} ${pxToRem(8)}`,
    borderRadius: "8px",
    cursor: "pointer",
    userSelect: "none",
    whiteSpace: "nowrap",
    position: "relative",
    borderLeft: active ? `2px solid #4D9FFF` : "2px solid transparent",
    transition: transitions.create(["background", "border-color", "color"], {
      easing: transitions.easing.easeInOut,
      duration: transitions.duration.shorter,
    }),

    "&:hover": {
      background: active ? "rgba(0,117,255,0.22)" : "rgba(255,255,255,0.04)",
    },

    [breakpoints.up("xl")]: {
      transition: transitions.create(["background", "border-color", "color", "box-shadow"], {
        easing: transitions.easing.easeInOut,
        duration: transitions.duration.shorter,
      }),
    },
  };
}

function collapseIconBox(theme, ownerState) {
  const { palette, transitions, breakpoints, functions } = theme;
  const { active } = ownerState;

  const { pxToRem } = functions;

  return {
    minWidth: pxToRem(36),
    minHeight: pxToRem(36),
    borderRadius: "8px",
    display: "grid",
    placeItems: "center",
    background: active ? "rgba(0,229,255,0.12)" : "transparent",
    transition: transitions.create("background", {
      easing: transitions.easing.easeInOut,
      duration: transitions.duration.standard,
    }),

    "& svg": {
      color: active ? palette.info.main : palette.text.main,
    },
  };
}

const collapseIcon = ({ palette: { info, text } }, { active }) => ({
  color: active ? info.main : text.main,
});

function collapseText(theme, ownerState) {
  const { typography, transitions, breakpoints, functions } = theme;
  const { miniSidenav, active } = ownerState;

  const { size, fontWeightMedium, fontWeightRegular } = typography;
  const { pxToRem } = functions;

  return {
    marginLeft: pxToRem(10),

    [breakpoints.up("xl")]: {
      opacity: miniSidenav ? 0 : 1,
      maxWidth: miniSidenav ? 0 : "100%",
      marginLeft: miniSidenav ? 0 : pxToRem(10),
      transition: transitions.create(["opacity", "margin"], {
        easing: transitions.easing.easeInOut,
        duration: transitions.duration.standard,
      }),
    },

    "& span": {
      fontWeight: active ? fontWeightMedium : fontWeightRegular,
      fontSize: size.sm,
      lineHeight: 0,
    },
  };
}

export { collapseItem, collapseIconBox, collapseIcon, collapseText };
