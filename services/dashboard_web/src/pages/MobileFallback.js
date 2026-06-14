// ============================================================================
// MobileFallback — 手机兜底页
// ============================================================================

import { useState } from "react";
import { Link } from "react-router-dom";
import { Card } from "@mui/material";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiButton from "components/VuiButton";
import VuiInput from "components/VuiInput";

function MobileFallback() {
  const [confirmText, setConfirmText] = useState("");
  const [confirmed, setConfirmed] = useState(false);

  const handleEmergencyClose = () => {
    if (confirmText === "确认") {
      setConfirmed(true);
      alert("紧急关闭指令已发送");
    }
  };

  if (confirmed) {
    return (
      <VuiBox display="flex" justifyContent="center" alignItems="center" minHeight="100vh" p={3}>
        <Card sx={{ p: 4, textAlign: "center", maxWidth: 400, width: "100%" }}>
          <VuiTypography variant="h4" color="error" fontWeight="bold" mb={2}>系统已关闭</VuiTypography>
          <VuiTypography color="text" mb={3}>请在桌面浏览器重新访问以恢复操作</VuiTypography>
          <Link to="/mobile" style={{ textDecoration: "none" }}>
            <VuiButton color="info" fullWidth>返回</VuiButton>
          </Link>
        </Card>
      </VuiBox>
    );
  }

  return (
    <VuiBox display="flex" justifyContent="center" alignItems="center" minHeight="100vh" p={3}>
      <Card sx={{ p: 4, textAlign: "center", maxWidth: 400, width: "100%" }}>
        <VuiTypography variant="h2" mb={2}>{"📱"}</VuiTypography>
        <VuiTypography variant="h5" fontWeight="bold" mb={1}>请在桌面浏览器访问</VuiTypography>
        <VuiTypography variant="body2" color="text" mb={4}>
          本系统优化于桌面端使用，请使用电脑浏览器打开以获最佳体验
        </VuiTypography>
        <VuiTypography variant="body2" color="text" mb={1}>
          查看{" "}
          <a href="/docs/ARCHITECTURE.md" target="_blank" rel="noopener noreferrer" style={{ color: "#0075FF" }}>
            Dashboard 文档
          </a>
        </VuiTypography>
        <VuiBox mt={4} pt={3} sx={{ borderTop: "1px solid rgba(255,255,255,0.1)" }}>
          <VuiTypography variant="button" color="error" fontWeight="bold" mb={2} display="block">紧急关闭</VuiTypography>
          <VuiInput placeholder='输入 "确认" 以授权' value={confirmText} onChange={(e) => setConfirmText(e.target.value)} sx={{ mb: 2 }} />
          <VuiButton color="error" variant="contained" size="large" fullWidth disabled={confirmText !== "确认"} onClick={handleEmergencyClose}>
            确认紧急关闭
          </VuiButton>
        </VuiBox>
      </Card>
    </VuiBox>
  );
}

export default MobileFallback;
