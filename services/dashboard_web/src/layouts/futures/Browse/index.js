// ============================================================================
// FuturesBrowse — 合约浏览（左列表 + 右详情大卡片）
// 点击合约 → 左侧卷轴收缩动画 → 右侧展开详情
// ============================================================================

import { useState, useMemo, useCallback } from "react";
import Grid from "@mui/material/Grid";
import Card from "@mui/material/Card";
import TextField from "@mui/material/TextField";
import InputAdornment from "@mui/material/InputAdornment";
import IconButton from "@mui/material/IconButton";
import Dialog from "@mui/material/Dialog";
import DialogTitle from "@mui/material/DialogTitle";
import DialogContent from "@mui/material/DialogContent";
import DialogActions from "@mui/material/DialogActions";
import Button from "@mui/material/Button";
import Icon from "@mui/material/Icon";
import { IoSearch, IoStar, IoWarning, IoClose } from "react-icons/io5";
import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiButton from "components/VuiButton";
import useTradingStore from "stores/trading";

// ---- Constants -------------------------------------------------------------
const M = { fontFamily: "'Roboto Mono', 'Courier New', monospace" };
const INTRA_PERIODS = ["1m", "5m", "15m", "30m", "60m"];
const KLINE_PERIODS = ["日K", "周K", "月K"];

// ---- Mock data -------------------------------------------------------------
const CC = [
  { s: "MA2501", n: "甲醇2501", p: 2524.0, c: 0.65, v: 12500 },
  { s: "MA2505", n: "甲醇2505", p: 2510.5, c: -0.32, v: 9850 },
  { s: "MA2509", n: "甲醇2509", p: 2498.0, c: 0.12, v: 6200 },
  { s: "MA2512", n: "甲醇2512", p: 2485.5, c: -0.55, v: 3100 },
  { s: "TA2501", n: "PTA2501", p: 5832.0, c: 0.28, v: 15200 },
  { s: "TA2505", n: "PTA2505", p: 5790.0, c: -0.41, v: 11800 },
  { s: "TA2509", n: "PTA2509", p: 5750.5, c: 0.05, v: 7200 },
  { s: "EG2501", n: "乙二醇2501", p: 4680.0, c: 1.20, v: 8900 },
  { s: "EG2505", n: "乙二醇2505", p: 4655.5, c: -0.78, v: 5400 },
  { s: "PF2503", n: "短纤2503", p: 7210.0, c: 0.33, v: 4100 },
];

const MA = [2526, 2525, 2524, 2523, 2522];
const MB = [2520, 2519, 2518, 2517, 2516];
const VA = [125, 340, 215, 180, 95];
const VB = [200, 310, 175, 420, 88];
const OB = {
  asks: MA.map((p, i) => ({ p, v: VA[i] })),
  bids: MB.map((p, i) => ({ p, v: VB[i] })),
};

function fmtPct(v) { return `${v >= 0 ? "+" : ""}${v.toFixed(2)}%`; }

// ---- Shared row style ------------------------------------------------------
function rowStyle(sel) {
  return {
    p: "6px 10px", mb: 0.3, borderRadius: "6px", cursor: "pointer",
    display: "flex", justifyContent: "space-between", alignItems: "center",
    bgcolor: sel ? "rgba(0,117,255,0.12)" : "transparent",
    transition: "background 0.15s",
    "&:hover": { bgcolor: "rgba(255,255,255,0.05)" },
  };
}

// ============================================================================
function FuturesBrowse() {
  const { positions, selectedSymbol, selectSymbol } = useTradingStore();
  const [ss, setSs] = useState("");
  const [eo, setEo] = useState(false);
  const [intraPeriod, setIntraPeriod] = useState("1m");
  const [klinePeriod, setKlinePeriod] = useState("日K");

  const q = ss.toLowerCase().trim();
  const fd = useMemo(() => q ? CC.filter(c => c.s.toLowerCase().includes(q) || c.n.toLowerCase().includes(q)) : CC, [q]);
  const pl = useMemo(() => Object.values(positions), [positions]);
  const open = !!selectedSymbol;

  const dt = useMemo(() => {
    if (!selectedSymbol) return null;
    const pos = positions[selectedSymbol] || null;
    const mc = CC.find(c => c.s === selectedSymbol);
    return {
      s: selectedSymbol,
      n: mc?.n || selectedSymbol,
      lp: pos?.lastPrice ?? mc?.p ?? 0,
      cp: pos?.changePct ?? mc?.c ?? 0,
      pos,
      ob: OB,
    };
  }, [selectedSymbol, positions]);

  const onEm = useCallback(m => {
    console.log("[EmergencyClose]", { mode: m, symbol: selectedSymbol });
    setEo(false);
  }, [selectedSymbol]);

  const onClose = () => selectSymbol(null);

  // ---- Contract list row ---------------------------------------------------
  const renderRow = (c) => (
    <VuiBox key={c.s} sx={rowStyle(selectedSymbol === c.s)} onClick={() => selectSymbol(c.s)}>
      <VuiBox>
        <VuiTypography variant="caption" color="white" fontWeight="medium" sx={M}>{c.s}</VuiTypography>
        {!open && <VuiTypography variant="caption" color="text" ml={1}>{c.n}</VuiTypography>}
      </VuiBox>
      <VuiBox sx={{ textAlign: "right" }}>
        <VuiTypography variant="caption" color="white" fontWeight="medium" sx={M}>{c.p.toFixed(1)}</VuiTypography>
        {!open && <VuiTypography variant="caption" color={c.c >= 0 ? "success" : "error"} sx={M} ml={1}>{fmtPct(c.c)}</VuiTypography>}
      </VuiBox>
    </VuiBox>
  );

  const renderPos = (p) => (
    <VuiBox key={p.symbol} sx={rowStyle(selectedSymbol === p.symbol)} onClick={() => selectSymbol(p.symbol)}>
      <VuiBox display="flex" alignItems="center" gap={0.5}>
        <IoStar size={12} color="#ffb547" />
        <VuiTypography variant="caption" color="white" fontWeight="medium" sx={M}>{p.symbol}</VuiTypography>
        {!open && <VuiTypography variant="caption" color="text">· {p.volume ?? "--"}手</VuiTypography>}
      </VuiBox>
      {!open && (
        <VuiTypography variant="caption" color={p.floatPnL >= 0 ? "success" : "error"} fontWeight="bold" sx={M}>
          {(p.floatPnL >= 0 ? "+" : "") + (p.floatPnL?.toFixed(0) ?? "0")}
        </VuiTypography>
      )}
    </VuiBox>
  );

  return (
    <DashboardLayout>
      <DashboardNavbar />
      <VuiBox py={3}>
        <VuiBox sx={{ display: "flex", gap: 1.5, height: "calc(100vh - 190px)" }}>
          {/* ================================================================ */}
          {/* LEFT PANEL — 卷轴收缩动画 */}
          {/* ================================================================ */}
          <VuiBox
            sx={{
              width: open ? 64 : 340,
              minWidth: open ? 64 : 340,
              flexShrink: 0,
              transition: "width 0.35s cubic-bezier(0.4, 0, 0.2, 1), min-width 0.35s cubic-bezier(0.4, 0, 0.2, 1)",
              overflow: "hidden",
            }}
          >
            <Card sx={{ p: open ? 1 : 2, height: "100%", display: "flex", flexDirection: "column" }}>
              {/* Search — 仅展开时显示 */}
              {!open && (
                <TextField fullWidth size="small" placeholder="搜索合约代码/名称..." value={ss}
                  onChange={e => setSs(e.target.value)}
                  InputProps={{ startAdornment: <InputAdornment position="start"><IoSearch size={16} color="#a0aec0" /></InputAdornment> }}
                  sx={{ mb: 2, "& .MuiOutlinedInput-root": { bgcolor: "rgba(15,21,53,0.6)" } }} />
              )}

              {/* Positions — 仅展开时显示详情 */}
              {!open && pl.length > 0 && (
                <VuiBox mb={2} pb={2} sx={{ borderBottom: "1px solid rgba(255,255,255,0.08)" }}>
                  <VuiTypography variant="button" color="white" fontWeight="bold" mb={1}
                    sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                    <IoStar size={14} color="#ffb547" /> 持仓中
                  </VuiTypography>
                  {pl.map(renderPos)}
                </VuiBox>
              )}

              {/* Contract list — 收缩模式下仅显示代码+最新价 */}
              {!open && (
                <VuiTypography variant="button" color="text" fontWeight="bold" mb={1}>全部合约</VuiTypography>
              )}
              <VuiBox sx={{ flex: 1, overflowY: "auto" }}>
                {open
                  // 收缩模式：紧凑列表
                  ? CC.map(c => (
                      <VuiBox key={c.s}
                        sx={{
                          py: 0.8, px: 0.5, mb: 0.1, borderRadius: "4px", cursor: "pointer",
                          bgcolor: selectedSymbol === c.s ? "rgba(0,117,255,0.15)" : "transparent",
                          textAlign: "center",
                          "&:hover": { bgcolor: "rgba(255,255,255,0.06)" },
                        }}
                        onClick={() => selectSymbol(c.s)}
                      >
                        <VuiTypography variant="caption" color="white" fontWeight="medium" fontSize="10px" sx={M}>
                          {c.s.replace(/\d+$/, "")}
                        </VuiTypography>
                        <VuiTypography variant="caption" color={c.c >= 0 ? "success" : "error"} fontSize="10px" sx={M}>
                          {c.p.toFixed(1)}
                        </VuiTypography>
                      </VuiBox>
                    ))
                  : fd.length === 0
                    ? <VuiTypography variant="caption" color="text" sx={{ textAlign: "center", py: 4, display: "block" }}>无匹配合约</VuiTypography>
                    : fd.map(renderRow)
                }
              </VuiBox>
            </Card>
          </VuiBox>

          {/* ================================================================ */}
          {/* RIGHT DETAIL — 有选中合约时才展示 */}
          {/* ================================================================ */}
          <VuiBox sx={{ flex: 1, minWidth: 0 }}>
            {!dt ? (
              <Card sx={{ p: 6, height: "100%", display: "flex", alignItems: "center", justifyContent: "center" }}>
                <VuiTypography variant="body1" color="text">← 选择合约查看详情</VuiTypography>
              </Card>
            ) : (
              <Card sx={{ p: 2.5, height: "100%", display: "flex", flexDirection: "column", position: "relative" }}>
                {/* ---- Header bar + Close button ---- */}
                <VuiBox sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", mb: 2, pb: 2, borderBottom: "1px solid rgba(255,255,255,0.08)" }}>
                  <VuiBox>
                    <VuiTypography variant="h5" color="white" fontWeight="bold" sx={M}>{dt.s}</VuiTypography>
                    <VuiTypography variant="caption" color="text">{dt.n}</VuiTypography>
                  </VuiBox>
                  <VuiBox sx={{ textAlign: "right", display: "flex", alignItems: "center", gap: 2 }}>
                    <VuiBox>
                      <VuiTypography variant="h3" color="white" fontWeight="bold" sx={M}>{dt.lp.toFixed(1)}</VuiTypography>
                      <VuiTypography variant="button" color={dt.cp >= 0 ? "success" : "error"} fontWeight="medium" sx={M}>
                        {fmtPct(dt.cp)}
                      </VuiTypography>
                    </VuiBox>
                    <IconButton onClick={onClose} size="small"
                      sx={{ color: "#a0aec0", "&:hover": { color: "white", bgcolor: "rgba(255,255,255,0.1)" } }}>
                      <IoClose size={22} />
                    </IconButton>
                  </VuiBox>
                </VuiBox>

                {/* ---- TOP ROW: 分时图 + 盘口五档（高度一致） ---- */}
                <VuiBox sx={{ display: "flex", gap: 1.5, height: "55%" }}>
                  {/* Intraday chart */}
                  <Card sx={{ flex: 1, display: "flex", flexDirection: "column", bgcolor: "rgba(0,0,0,0.25)", p: 1.5 }}>
                    <VuiBox sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 1 }}>
                      <VuiTypography variant="caption" color="text" fontWeight="bold" sx={M}>分时图</VuiTypography>
                      <VuiBox sx={{ display: "flex", gap: 0.3 }}>
                        {INTRA_PERIODS.map(p => (
                          <VuiButton key={p} color={intraPeriod === p ? "info" : "dark"} size="small"
                            onClick={() => setIntraPeriod(p)}
                            sx={{ minWidth: 36, px: 1, py: 0.2, fontSize: "11px", color: intraPeriod === p ? "white" : "text" }}>
                            {p}
                          </VuiButton>
                        ))}
                      </VuiBox>
                    </VuiBox>
                    <VuiBox sx={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center" }}>
                      <VuiTypography variant="caption" color="text" sx={M}>
                        {intraPeriod} · Lightweight Charts
                      </VuiTypography>
                    </VuiBox>
                  </Card>

                  {/* Orderbook — 与分时图等高 */}
                  <Card sx={{ width: 220, flexShrink: 0, display: "flex", flexDirection: "column", bgcolor: "rgba(0,0,0,0.25)", p: 1.5 }}>
                    <VuiTypography variant="caption" color="text" fontWeight="bold" mb={1}>盘口五档</VuiTypography>
                    <VuiBox sx={{ flex: 1, display: "flex", flexDirection: "column", justifyContent: "center" }}>
                      {dt.ob.asks.slice().reverse().map(r => (
                        <VuiBox key={`a${r.p}`} sx={{ display: "flex", justifyContent: "space-between", py: 0.3 }}>
                          <VuiTypography variant="caption" color="error" sx={M}>{r.p.toFixed(1)}</VuiTypography>
                          <VuiTypography variant="caption" color="text" sx={M}>{r.v}</VuiTypography>
                        </VuiBox>
                      ))}
                      <VuiBox sx={{ borderTop: "1px solid rgba(255,255,255,0.1)", borderBottom: "1px solid rgba(255,255,255,0.1)", py: 0.4, my: 0.4, textAlign: "center" }}>
                        <VuiTypography variant="body2" color="info" fontWeight="bold" sx={M}>{dt.lp.toFixed(1)}</VuiTypography>
                      </VuiBox>
                      {dt.ob.bids.map(r => (
                        <VuiBox key={`b${r.p}`} sx={{ display: "flex", justifyContent: "space-between", py: 0.3 }}>
                          <VuiTypography variant="caption" color="success" sx={M}>{r.p.toFixed(1)}</VuiTypography>
                          <VuiTypography variant="caption" color="text" sx={M}>{r.v}</VuiTypography>
                        </VuiBox>
                      ))}
                    </VuiBox>
                  </Card>
                </VuiBox>

                {/* ---- BOTTOM ROW: K线图 + 持仓（铺满剩余空间） ---- */}
                <VuiBox sx={{ display: "flex", gap: 1.5, flex: 1, mt: 1.5, minHeight: 0 }}>
                  {/* K-line — 占剩余宽度 */}
                  <Card sx={{ flex: 1, display: "flex", flexDirection: "column", bgcolor: "rgba(0,0,0,0.25)", p: 1.5 }}>
                    <VuiBox sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 1 }}>
                      <VuiTypography variant="caption" color="text" fontWeight="bold" sx={M}>K 线图</VuiTypography>
                      <VuiBox sx={{ display: "flex", gap: 0.3 }}>
                        {KLINE_PERIODS.map(p => (
                          <VuiButton key={p} color={klinePeriod === p ? "info" : "dark"} size="small"
                            onClick={() => setKlinePeriod(p)}
                            sx={{ minWidth: 40, px: 1, py: 0.2, fontSize: "11px", color: klinePeriod === p ? "white" : "text" }}>
                            {p}
                          </VuiButton>
                        ))}
                      </VuiBox>
                    </VuiBox>
                    <VuiBox sx={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center" }}>
                      <VuiTypography variant="caption" color="text" sx={M}>
                        {klinePeriod} · Lightweight Charts
                      </VuiTypography>
                    </VuiBox>
                  </Card>

                  {/* Position + Emergency — 与盘口等宽 */}
                  {dt.pos ? (
                    <Card sx={{ width: 220, flexShrink: 0, display: "flex", flexDirection: "column", bgcolor: "rgba(0,0,0,0.25)", p: 1.5 }}>
                      <VuiTypography variant="caption" color="text" fontWeight="bold" mb={1}>我的持仓</VuiTypography>
                      <VuiBox sx={{ flex: 1 }}>
                        <VuiBox sx={{ display: "flex", justifyContent: "space-between", mb: 0.5 }}>
                          <VuiTypography variant="caption" color="text">方向 / 手数</VuiTypography>
                          <VuiTypography variant="caption" color={dt.pos.volume > 0 ? "success" : "error"} fontWeight="bold" sx={M}>
                            {dt.pos.volume > 0 ? "多 " : "空 "}{Math.abs(dt.pos.volume)}
                          </VuiTypography>
                        </VuiBox>
                        <VuiBox sx={{ display: "flex", justifyContent: "space-between", mb: 1 }}>
                          <VuiTypography variant="caption" color="text">浮动盈亏</VuiTypography>
                          <VuiTypography variant="caption" color={dt.pos.floatPnL >= 0 ? "success" : "error"} fontWeight="bold" sx={M}>
                            {(dt.pos.floatPnL >= 0 ? "+¥" : "-¥") + Math.abs(dt.pos.floatPnL ?? 0).toFixed(0)}
                          </VuiTypography>
                        </VuiBox>
                      </VuiBox>
                      <Button fullWidth variant="contained" color="error" size="medium" startIcon={<IoWarning />}
                        onClick={() => setEo(true)}
                        sx={{ fontWeight: "bold", fontSize: "0.8rem", mt: "auto" }}>
                        紧急平仓
                      </Button>
                    </Card>
                  ) : (
                    <Card sx={{ width: 220, flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", bgcolor: "rgba(0,0,0,0.25)", p: 1.5 }}>
                      <VuiTypography variant="caption" color="text">无持仓</VuiTypography>
                    </Card>
                  )}
                </VuiBox>
              </Card>
            )}
          </VuiBox>
        </VuiBox>

        {/* ================================================================ */}
        {/* Emergency Dialog */}
        {/* ================================================================ */}
        <Dialog open={eo} onClose={() => setEo(false)}
          PaperProps={{ sx: { bgcolor: "#1a1f37", borderRadius: "16px", minWidth: 360, p: 1 } }}>
          <DialogTitle sx={{ color: "#e31a1a", fontWeight: "bold", fontSize: "1.1rem", textAlign: "center" }}>
            紧急平仓确认
          </DialogTitle>
          <DialogContent sx={{ color: "#a0aec0", textAlign: "center", py: 1 }}>
            请选择平仓操作方式
          </DialogContent>
          <DialogActions sx={{ justifyContent: "center", gap: 1, pb: 2 }}>
            <Button variant="outlined" color="error" onClick={() => onEm("close_only")}>仅平本合约</Button>
            <Button variant="contained" color="error" onClick={() => onEm("close_all_pause")}>全部平仓+暂停</Button>
          </DialogActions>
        </Dialog>
      </VuiBox>
    </DashboardLayout>
  );
}

export default FuturesBrowse;
