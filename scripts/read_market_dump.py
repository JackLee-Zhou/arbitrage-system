#!/usr/bin/env python3
"""
Read and inspect Fuyao market-dump Parquet files for both publishable kinds:

  - daily-k             rows keyed by (thscode, date_ms),     dump_id a_share_daily_k_1d_none_1y
  - adjustment-factors  rows keyed by (thscode, ex_date_ms),  dump_id a_share_adjustment_factors_event_none_all

The kind is auto-detected from the Parquet schema (presence of `ex_date_ms`),
so the same CLI works for either dump.

下载来源: https://fuyao.aicubes.cn/docs/api-reference/market-dumps
  - 需要用同花顺账号登录，在浏览器点击「获取下载链接」按钮
  - 预签名链接 5 分钟有效，拿到后立即下载
  - 日K dump_id: a_share_daily_k_1d_none_10y（10年）或 a_share_daily_k_1d_none_1y（1年）
  - 复权因子 dump_id: a_share_adjustment_factors_event_none_all

---- 文件结构 ----
日K (daily-k) Parquet schema:
  thscode     string   -- 带交易所后缀，如 600519.SH
  currency    string   -- 币种，A 股固定 CNY
  interval    string   -- 周期，固定 1d（仅日线，无分钟级）
  adjusted    string   -- 复权方式，固定 none（未复权）
  date_ms     int64    -- K 线日期，毫秒 Unix 时间戳，Asia/Shanghai 时区零点
  open_price  double   -- 开盘价（原始货币）
  high_price  double   -- 最高价
  low_price   double   -- 最低价
  close_price double   -- 收盘价
  volume      double   -- 成交量（股）
  turnover    double   -- 成交额（原始货币）

复权因子 (adjustment-factors) Parquet schema:
  thscode              string   -- 带交易所后缀
  ticker               string   -- 纯代码（无后缀），如 600519
  ex_date_ms           int64    -- 除权除息日，毫秒时间戳
  dividend_per_share   number   -- 每股现金分红（税前）
  per_share_bonus      number   -- 每股送股比例
  allotment_ratio      number   -- 配股比例
  allotment_price      number   -- 配股价格
  currency             string   -- CNY

实测文件（2026-06-18 下载）:
  文件名: a_share_daily_k_1d_none_10y_20260618.parquet
  大小:   170 MB
  行数:   ~1010 万行
  股票数: 5,521 只
  跨度:   约 10 年（2016-06 ~ 2026-06）

---- 快速解析（Python 代码片段） ----
```python
import pyarrow.parquet as pq
from datetime import datetime, timezone, timedelta

zone = timezone(timedelta(hours=8))  # Asia/Shanghai
pf = pq.ParquetFile('a_share_daily_k_1d_none_10y_20260618.parquet')

# 方式1: 遍历所有行（流式，省内存）
for batch in pf.iter_batches(batch_size=65536):
    df = batch.to_pandas()
    # ... 处理 df

# 方式2: 全部读入内存（约 300-500 MB 解压后）
df = pf.read().to_pandas()

# 方式3: 只看某只股票（流式过滤）
target = '600519.SH'
for batch in pf.iter_batches(batch_size=65536):
    df = batch.to_pandas()
    mask = df['thscode'] == target
    rows = df[mask]
    for _, r in rows.iterrows():
        dt = datetime.fromtimestamp(r['date_ms'] / 1000, tz=zone)
        print(f"{dt:%Y-%m-%d} O:{r['open_price']} H:{r['high_price']} "
              f"L:{r['low_price']} C:{r['close_price']} V:{r['volume']}")

# 方式4: 用 pandas 过滤 + 排序
df = pf.read().to_pandas()
moutai = df[df['thscode'] == '600519.SH'].sort_values('date_ms')
# 最近 5 天
recent = moutai.tail(5)
recent['date'] = pd.to_datetime(recent['date_ms'], unit='ms', utc=True)
recent['date'] = recent['date'].dt.tz_convert(zone)
print(recent[['date', 'open_price', 'high_price', 'low_price', 'close_price', 'volume']])
```

---- CLI 使用示例 ----
  # 查看 schema 和摘要
  python3 scripts/read_market_dump.py a_share_daily_k_1d_none_10y_20260618.parquet --no-sample

  # 只看茅台最近数据
  python3 scripts/read_market_dump.py a_share_daily_k_1d_none_10y_20260618.parquet --ticker 600519.SH --limit 10

  # 过滤日期范围 + 导出 CSV
  python3 scripts/read_market_dump.py a_share_daily_k_1d_none_10y_20260618.parquet \
    --from-date 2025-06-01 --to-date 2026-06-18 \
    --export-csv ~/volumes/西数/fuyao_market_dump/export.csv

  # 检查重复
  python3 scripts/read_market_dump.py a_share_daily_k_1d_none_10y_20260618.parquet --check-duplicates

---- 依赖 ----
  pip install pyarrow
"""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import sys
from collections import defaultdict
from dataclasses import dataclass
from datetime import date, datetime, time, timedelta
from pathlib import Path
from typing import Any
from zoneinfo import ZoneInfo


ZONE = ZoneInfo("Asia/Shanghai")
DEFAULT_COLUMNS = [
    "thscode",
    "currency",
    "interval",
    "adjusted",
    "date_ms",
    "open_price",
    "high_price",
    "low_price",
    "close_price",
    "volume",
    "turnover",
]
ADJUSTMENT_FACTOR_COLUMNS = [
    "thscode",
    "ticker",
    "ex_date_ms",
    "dividend_per_share",
    "per_share_bonus",
    "allotment_ratio",
    "allotment_price",
    "currency",
]


@dataclass(frozen=True)
class ResolvedInput:
    manifest_path: Path | None
    parquet_path: Path
    manifest: dict[str, Any] | None


def import_pyarrow():
    try:
        import pyarrow.parquet as pq  # type: ignore
    except ModuleNotFoundError:
        print(
            "ERROR: reading Parquet requires pyarrow.\n"
            "Install it with:\n"
            "  python3 -m pip install pyarrow\n",
            file=sys.stderr,
        )
        raise SystemExit(1)
    return pq


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Read and inspect Fuyao market dump Parquet files.",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "path",
        help=(
            "Path to manifest.json, a version directory containing manifest.json, "
            "or a .parquet file."
        ),
    )
    parser.add_argument(
        "--prefer",
        choices=["current", "compact"],
        default="compact",
        help=(
            "When reading a manifest, prefer compact_file if present, otherwise "
            "use the manifest current file_name."
        ),
    )
    parser.add_argument("--ticker", action="append", help="Filter by thscode. Can be repeated.")
    parser.add_argument("--from-date", help="Filter date_ms from this YYYY-MM-DD date, inclusive.")
    parser.add_argument("--to-date", help="Filter date_ms to this YYYY-MM-DD date, inclusive.")
    parser.add_argument("--limit", type=int, default=20, help="Number of sample rows to print.")
    parser.add_argument("--batch-size", type=int, default=65536, help="Parquet streaming batch size.")
    parser.add_argument(
        "--check-duplicates",
        action="store_true",
        help="Check duplicate (thscode,date_ms) keys. This keeps keys in memory.",
    )
    parser.add_argument(
        "--no-sample",
        action="store_true",
        help="Only print schema and summary, without sample rows.",
    )
    parser.add_argument(
        "--export-csv",
        help="Export filtered rows to CSV. Useful for downstream users without Parquet tooling.",
    )
    parser.add_argument(
        "--skip-sha256",
        action="store_true",
        help="Skip sha256 validation against manifest.",
    )
    return parser.parse_args()


def load_json(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def resolve_input(raw_path: str, prefer: str) -> ResolvedInput:
    path = Path(raw_path)
    if not path.exists():
        raise SystemExit(f"ERROR: path does not exist: {path}")

    if path.is_dir():
        manifest_path = path / "manifest.json"
        if not manifest_path.is_file():
            raise SystemExit(f"ERROR: directory does not contain manifest.json: {path}")
        return resolve_from_manifest(manifest_path, prefer)

    if path.name == "manifest.json":
        return resolve_from_manifest(path, prefer)

    if path.suffix == ".parquet":
        return ResolvedInput(manifest_path=None, parquet_path=path, manifest=None)

    raise SystemExit(
        "ERROR: path must be a manifest.json, a version directory, or a .parquet file"
    )


def resolve_from_manifest(manifest_path: Path, prefer: str) -> ResolvedInput:
    manifest = load_json(manifest_path)
    dump_root = manifest_path.parent.parent

    rel_path = manifest.get("file_name")
    if prefer == "compact" and manifest.get("compact_file"):
        rel_path = manifest["compact_file"].get("path")

    if not rel_path:
        raise SystemExit(f"ERROR: manifest has no readable file path: {manifest_path}")

    parquet_path = dump_root / rel_path
    if not parquet_path.is_file():
        raise SystemExit(f"ERROR: parquet file referenced by manifest does not exist: {parquet_path}")

    return ResolvedInput(
        manifest_path=manifest_path,
        parquet_path=parquet_path,
        manifest=manifest,
    )


def parse_day(raw: str | None, end_of_day: bool) -> int | None:
    if not raw:
        return None
    d = date.fromisoformat(raw)
    if end_of_day:
        dt = datetime.combine(d + timedelta(days=1), time.min, ZONE) - timedelta(milliseconds=1)
    else:
        dt = datetime.combine(d, time.min, ZONE)
    return int(dt.timestamp() * 1000)


def row_passes_filters(
    row: dict[str, Any],
    date_column: str,
    tickers: set[str] | None,
    from_ms: int | None,
    to_ms: int | None,
) -> bool:
    if tickers is not None and row.get("thscode") not in tickers:
        return False
    date_ms = row.get(date_column)
    if from_ms is not None and date_ms < from_ms:
        return False
    if to_ms is not None and date_ms > to_ms:
        return False
    return True


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def print_manifest_summary(resolved: ResolvedInput, skip_sha256: bool) -> None:
    manifest = resolved.manifest
    if not manifest:
        print("manifest: <not provided>")
        print(f"parquet : {resolved.parquet_path}")
        return

    print(f"manifest: {resolved.manifest_path}")
    print(f"parquet : {resolved.parquet_path}")
    print("manifest_summary:")
    for key in [
        "dump_id",
        "version",
        "mode",
        "coverage_start_date",
        "coverage_end_date",
        "start_date",
        "end_date",
        "adjusted",
        "row_count",
        "ticker_count",
        "failed_tickers",
        "file_name",
    ]:
        print(f"  {key}: {manifest.get(key)}")

    expected = manifest.get("sha256")
    if expected and not skip_sha256:
        actual = sha256(resolved.parquet_path)
        print(f"  sha256: {actual}")
        if actual != expected:
            raise SystemExit(f"ERROR: sha256 mismatch, expected={expected}, actual={actual}")
        print("  sha256_check: OK")


def print_schema(pq: Any, parquet_path: Path) -> None:
    parquet_file = pq.ParquetFile(parquet_path)
    print("schema:")
    print(parquet_file.schema_arrow)


def inspect_rows(
    pq: Any,
    resolved: ResolvedInput,
    tickers: set[str] | None,
    from_ms: int | None,
    to_ms: int | None,
    limit: int,
    batch_size: int,
    check_duplicates: bool,
    export_csv_path: str | None,
    no_sample: bool,
) -> None:
    parquet_file = pq.ParquetFile(resolved.parquet_path)
    schema_names = parquet_file.schema_arrow.names
    if "ex_date_ms" in schema_names:
        selected_columns = [name for name in ADJUSTMENT_FACTOR_COLUMNS if name in schema_names]
        date_column = "ex_date_ms"
    else:
        selected_columns = [name for name in DEFAULT_COLUMNS if name in schema_names]
        date_column = "date_ms"
    has_adjusted_column = "adjusted" in selected_columns

    total_rows = 0
    filtered_rows = 0
    bad_adjusted_count = 0
    sample_rows: list[dict[str, Any]] = []
    stats: dict[str, dict[str, Any]] = defaultdict(
        lambda: {"rows": 0, "min_date_ms": None, "max_date_ms": None}
    )
    seen_keys: set[tuple[str, int]] = set()
    duplicate_keys: list[tuple[str, int]] = []

    csv_file = None
    writer = None
    if export_csv_path:
        csv_file = Path(export_csv_path).open("w", encoding="utf-8", newline="")
        writer = csv.DictWriter(csv_file, fieldnames=selected_columns)
        writer.writeheader()

    try:
        for batch in parquet_file.iter_batches(batch_size=batch_size, columns=selected_columns):
            for row in batch.to_pylist():
                total_rows += 1
                if not row_passes_filters(row, date_column, tickers, from_ms, to_ms):
                    continue

                filtered_rows += 1
                thscode = row["thscode"]
                key_ts = row[date_column]  # date_ms for daily-k, ex_date_ms for adjustment-factors
                if has_adjusted_column and row.get("adjusted") != "none":
                    bad_adjusted_count += 1

                item = stats[thscode]
                item["rows"] += 1
                item["min_date_ms"] = key_ts if item["min_date_ms"] is None else min(item["min_date_ms"], key_ts)
                item["max_date_ms"] = key_ts if item["max_date_ms"] is None else max(item["max_date_ms"], key_ts)

                if check_duplicates:
                    key = (thscode, key_ts)
                    if key in seen_keys and len(duplicate_keys) < 20:
                        duplicate_keys.append(key)
                    seen_keys.add(key)

                if writer:
                    writer.writerow({name: row.get(name) for name in selected_columns})

                if not no_sample and len(sample_rows) < limit:
                    sample_rows.append(row)
    finally:
        if csv_file:
            csv_file.close()

    print("data_summary:")
    print(f"  parquet_rows_scanned: {total_rows}")
    print(f"  filtered_rows: {filtered_rows}")
    print(f"  filtered_ticker_count: {len(stats)}")
    print(f"  bad_adjusted_count: {bad_adjusted_count}")

    manifest_row_count = resolved.manifest.get("row_count") if resolved.manifest else None
    if tickers is None and from_ms is None and to_ms is None and manifest_row_count is not None:
        if filtered_rows != manifest_row_count:
            raise SystemExit(
                f"ERROR: manifest row_count {manifest_row_count} != parquet row_count {filtered_rows}"
            )
        print("  manifest_row_count_check: OK")

    if bad_adjusted_count:
        raise SystemExit(f"ERROR: found rows whose adjusted != none, count={bad_adjusted_count}")

    print("rows_by_ticker:")
    for thscode in sorted(stats):
        item = stats[thscode]
        print(
            f"  {thscode}: rows={item['rows']}, "
            f"min_date_ms={item['min_date_ms']}, max_date_ms={item['max_date_ms']}"
        )

    if check_duplicates:
        if duplicate_keys:
            print("duplicate_keys_sample:")
            for thscode, key_ts in duplicate_keys:
                print(f"  {thscode}, {key_ts}")
            raise SystemExit(
                f"ERROR: duplicate (thscode,{date_column}) keys found, "
                f"sample_count={len(duplicate_keys)}"
            )
        print(f"duplicate_key_check: OK (keyed on thscode,{date_column})")

    if export_csv_path:
        print(f"csv_exported: {export_csv_path}")

    if not no_sample:
        print("sample_rows:")
        for row in sample_rows:
            print(json.dumps(row, ensure_ascii=False, default=str))


def main() -> None:
    args = parse_args()
    pq = import_pyarrow()
    resolved = resolve_input(args.path, args.prefer)
    tickers = set(args.ticker) if args.ticker else None
    from_ms = parse_day(args.from_date, end_of_day=False)
    to_ms = parse_day(args.to_date, end_of_day=True)

    print_manifest_summary(resolved, args.skip_sha256)
    print_schema(pq, resolved.parquet_path)
    inspect_rows(
        pq=pq,
        resolved=resolved,
        tickers=tickers,
        from_ms=from_ms,
        to_ms=to_ms,
        limit=args.limit,
        batch_size=args.batch_size,
        check_duplicates=args.check_duplicates,
        export_csv_path=args.export_csv,
        no_sample=args.no_sample,
    )


if __name__ == "__main__":
    main()
