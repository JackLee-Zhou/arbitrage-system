// OPT-304: 期货委托页 — 当日委托列表
// 表格 + 状态筛选 tabs + 空态提示

import { useState, useMemo } from "react";

import DashboardLayout from "examples/LayoutContainers/DashboardLayout";
import DashboardNavbar from "examples/Navbars/DashboardNavbar";
import VuiBox from "components/VuiBox";
import VuiTypography from "components/VuiTypography";
import VuiButton from "components/VuiButton";

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

const FILTERS = ["全部", "待成交", "已成交", "已撤销"];

const FILTER_STATUS_MAP = {
  "待成交": "pending",
  "已成交": "filled",
  "已撤销": "canceled",
};

const STATUS_CONFIG = {
  pending:  { label: "待成交", color: "orange"  },
  filled:   { label: "已成交", color: "success" },
  canceled: { label: "已撤销", color: "text"    },
};

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

// ---------------------------------------------------------------------------
// Column definitions
// ---------------------------------------------------------------------------

const COLUMNS = [
  { key: "time",         label: "时间",   align: "left"   },
  { key: "symbol",       label: "合约",   align: "left"   },
  { key: "direction",    label: "方向",   align: "center" },
  { key: "price",        label: "价格",   align: "right"  },
  { key: "volume",       label: "数量",   align: "right"  },
  { key: "status",       label: "状态",   align: "center" },
  { key: "filledVolume", label: "成交数量", align: "right" },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

function FuturesOrders() {
  const { todayOrders } = useTradingStore();
  const [filter, setFilter] = useState("全部");

  const { grey } = colors;
  const { size, fontWeightBold } = typography;
  const { borderWidth } = borders;

  // ---- filter logic -------------------------------------------------------
  const filteredOrders = useMemo(() => {
    if (filter === "全部") return todayOrders;
    const target = FILTER_STATUS_MAP[filter];
    return todayOrders.filter((o) => o.status === target);
  }, [todayOrders, filter]);

  // ---- render -------------------------------------------------------------
  return (
    <DashboardLayout>
      <DashboardNavbar />

      <VuiBox py={3}>
        <Card sx={{ overflow: "visible" }}>
          {/* ---- Header + Filter Tabs ---- */}
          <VuiBox
            display="flex"
            justifyContent="space-between"
            alignItems="center"
            flexWrap="wrap"
            px={3}
            pt={3}
            pb={2}
          >
            <VuiTypography variant="lg" color="white" fontWeight="bold">
              当日委托
            </VuiTypography>

            <VuiBox display="flex" gap={1}>
              {FILTERS.map((f) => (
                <VuiButton
                  key={f}
                  variant={filter === f ? "contained" : "outlined"}
                  color={filter === f ? "info" : "white"}
                  size="small"
                  circular
                  onClick={() => setFilter(f)}
                  sx={{ minWidth: 56 }}
                >
                  {f}
                </VuiButton>
              ))}
            </VuiBox>
          </VuiBox>

          {/* ---- Body ---- */}
          {filteredOrders.length === 0 ? (
            <VuiBox textAlign="center" py={8}>
              <VuiTypography variant="body1" color="text">
                当日暂无委托
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
                  {filteredOrders.map((order, rowIdx) => (
                    <TableRow key={order.id || `order-${rowIdx}`}>
                      {COLUMNS.map((col) => {
                        // resolve cell value
                        let value;
                        switch (col.key) {
                          case "time":
                            value = formatTime(order.timestamp);
                            break;
                          case "direction":
                            value =
                              order.direction === "buy"
                                ? "买"
                                : order.direction === "sell"
                                  ? "卖"
                                  : order.direction || "--";
                            break;
                          case "price":
                            value = formatPrice(order.price);
                            break;
                          case "status": {
                            const cfg = STATUS_CONFIG[order.status];
                            value = cfg ? cfg.label : order.status || "--";
                            break;
                          }
                          case "filledVolume":
                            value = order.filledVolume != null ? order.filledVolume : "--";
                            break;
                          default:
                            value = order[col.key] ?? "--";
                        }

                        // direction buys/sells also get colored
                        const isDirection = col.key === "direction";
                        const dirColor =
                          order.direction === "buy"
                            ? "error"
                            : order.direction === "sell"
                              ? "success"
                              : null;

                        const isStatus = col.key === "status";
                        const statusColor = STATUS_CONFIG[order.status]?.color || null;

                        return (
                          <VuiBox
                            key={col.key}
                            component="td"
                            p={1}
                            textAlign={col.align}
                            borderBottom={
                              rowIdx < filteredOrders.length - 1
                                ? `${borderWidth[1]} solid ${grey[700]}`
                                : "none"
                            }
                          >
                            <VuiTypography
                              variant="button"
                              fontWeight={isStatus ? "bold" : "regular"}
                              color={
                                isStatus
                                  ? statusColor
                                  : isDirection
                                    ? dirColor
                                    : "text"
                              }
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

export default FuturesOrders;
