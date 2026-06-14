// Simple logo placeholder — used by Sidenav brand area
import { Icon } from "@mui/material";

export default function SimmmpleLogo({ size = "24px" }) {
  return (
    <Icon
      sx={{
        fontSize: size,
        background: "linear-gradient(135deg, #0075FF 0%, #9D5FFF 100%)",
        borderRadius: "8px",
        width: size,
        height: size,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        color: "white",
        fontWeight: "bold",
      }}
    >
      A
    </Icon>
  );
}
