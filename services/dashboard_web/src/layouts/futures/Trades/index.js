// OPT-304: 期货成交页 — 当日成交明细
// 表格 + 空态提示

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";

import { Card } from "@mui/material";
import MuiTable from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableContainer from "@mui/material/TableContainer";
import TableRow from "@mui/material/TableRow";

import useTradingStore from "stores/trading";

import colors from "assets/theme/base/colors";
import typography from "assets/theme/base/typography";
import borders from "assets/theme/base/borders";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTime(ts) {
  if (!ts) return "--";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  return d.toTimeString().slice(0, 8);
}

function formatPrice(price) {
  if (price == null) return "--";
  return price < 100 ? price.toFixed(2) : price.toFixed(1);
}

function formatCommission(fee) {
  if (fee == null) return "--";
  return Number(fee).toFixed(2);
}

// ---------------------------------------------------------------------------
// Column definitions
// ---------------------------------------------------------------------------

const COLUMNS = [
  { key: "time",       label: "时间",   align: "left"   },
  { key: "symbol",     label: "合约",   align: "left"   },
  { key: "direction",  label: "方向",   align: "center" },
  { key: "price",      label: "价格",   align: "right"  },
  { key: "volume",     label: "数量",   align: "right"  },
  { key: "commission", label: "手续费", align: "right"  },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

function FuturesTrades() {
  const { todayTrades } = useTradingStore();

  const { grey } = colors;
  const { size, fontWeightBold } = typography;
  const { borderWidth } = borders;

  return (
    <DashboardLayout>
      <DashboardNavbar />

      <VuiBox py={3}>
        <Card sx={{ overflow: "visible" }}>
          {/* ---- Header ---- */}
          <VuiBox px={3} pt={3} pb={2}>
            <VuiTypography variant="lg" color="white" fontWeight="bold">
              当日成交
            </VuiTypography>
          </VuiBox>

          {/* ---- Body ---- */}
          {todayTrades.length === 0 ? (
            <VuiBox textAlign="center" py={8}>
              <VuiTypography variant="body1" color="text">
                当日暂无成交
              </VuiTypography>
            </VuiBox>
          ) : (
            <TableContainer>
              <MuiTable>
                {/* ---- Thead ---- */}
                <VuiBox component="thead">
                  <TableRow>
                    {COLUMNS.map((col, i) => (
                      <VuiBox
                        key={col.key}
                        component="th"
                        pt={1.5}
                        pb={1.25}
                        pl={i === 0 ? 3 : 1}
                        pr={i === COLUMNS.length - 1 ? 3 : 1}
                        textAlign={col.align}
                        fontSize={size.xxs}
                        fontWeight={fontWeightBold}
                        color="text"
                        opacity={0.7}
                        borderBottom={`${borderWidth[1]} solid ${grey[700]}`}
                      >
                        {col.label}
                      </VuiBox>
                    ))}
                  </TableRow>
                </VuiBox>

                {/* ---- Tbody ---- */}
                <TableBody>
                  {todayTrades.map((trade, rowIdx) => (
                    <TableRow key={trade.id || `trade-${rowIdx}`}>
                      {COLUMNS.map((col) => {
                        // resolve cell value
                        let value;
                        switch (col.key) {
                          case "time":
                            value = formatTime(trade.timestamp);
                            break;
                          case "direction":
                            value =
                              trade.direction === "buy"
                                ? "买"
                                : trade.direction === "sell"
                                  ? "卖"
                                  : trade.direction || "--";
                            break;
                          case "price":
                            value = formatPrice(trade.price);
                            break;
                          case "commission":
                            value = formatCommission(trade.commission ?? trade.fee);
                            break;
                          default:
                            value = trade[col.key] ?? "--";
                        }

                        // direction colours
                        const isDirection = col.key === "direction";
                        const dirColor =
                          trade.direction === "buy"
                            ? "error"
                            : trade.direction === "sell"
                              ? "success"
                              : null;

                        return (
                          <VuiBox
                            key={col.key}
                            component="td"
                            p={1}
                            textAlign={col.align}
                            borderBottom={
                              rowIdx < todayTrades.length - 1
                                ? `${borderWidth[1]} solid ${grey[700]}`
                                : "none"
                            }
                          >
                            <VuiTypography
                              variant="button"
                              fontWeight="regular"
                              color={isDirection ? dirColor : "text"}
                              sx={{ display: "inline-block", width: "max-content" }}
                            >
                              {value}
                            </VuiTypography>
                          </VuiBox>
                        );
                      })}
                    </TableRow>
                  ))}
                </TableBody>
              </MuiTable>
            </TableContainer>
          )}
        </Card>
      </VuiBox>
    </DashboardLayout>
  );
}

export default FuturesTrades;
