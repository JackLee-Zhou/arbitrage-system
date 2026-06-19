#!/usr/bin/env python3
"""
从 fuyao.aicubes.cn 下载全市场行情 Parquet 文件，并导出为 CSV。

用法:
  # 下载日K数据并导出CSV
  python3 scripts/download_market_dump.py daily-k --export-csv

  # 下载复权因子数据并导出CSV
  python3 scripts/download_market_dump.py adjustment-factors --export-csv

  # 只下载Parquet，不导出CSV
  python3 scripts/download_market_dump.py daily-k

  # 指定输出目录（默认 ~/volumes/西数/fuyao_market_dump）
  python3 scripts/download_market_dump.py daily-k --output-dir /path/to/dir

  # 只解析已有的 Parquet 文件
  python3 scripts/download_market_dump.py daily-k --parquet-file /path/to/file.parquet --export-csv

鉴权说明:
  全市场数据导出使用浏览器 Cookie 鉴权（非 X-api-key），需要手动下载。
  脚本会：
    1. 打开浏览器访问下载页面获取预签名链接
    2. 或引导你手动下载后将 Parquet 放到指定目录
    3. 自动解析 Parquet 并导出 CSV

依赖:
  pip install pyarrow pandas
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import time
from datetime import datetime
from pathlib import Path
from urllib.parse import urlparse

# --- 配置 ---
BASE_URL = "https://fuyao.aicubes.cn"
DUMP_KINDS = {
    "daily-k": {
        "dump_id": "a_share_daily_k_1d_none_1y",
        "description": "A 股全市场日 K 线（近 1 年，未复权）",
        "download_page": f"{BASE_URL}/docs/api-reference/market-dumps",
    },
    "adjustment-factors": {
        "dump_id": "a_share_adjustment_factors_event_none_all",
        "description": "A 股全市场复权因子（全历史）",
        "download_page": f"{BASE_URL}/docs/api-reference/market-dumps",
    },
}
DEFAULT_OUTPUT_DIR = os.path.expanduser("~/volumes/西数/fuyao_market_dump")


def ensure_dir(path: Path) -> None:
    """创建目录（如不存在）。"""
    path.mkdir(parents=True, exist_ok=True)
    print(f"📁 输出目录: {path}")


def try_programmatic_download(kind: str, output_dir: Path, api_key: str | None) -> Path | None:
    """
    尝试通过 API 获取下载链接并下载 Parquet 文件。

    注意：文档标注该端点使用 Cookie 鉴权，X-api-key 可能不生效。
    如果 API Key 可用，返回下载文件路径；否则返回 None。
    """
    dump_id = DUMP_KINDS[kind]["dump_id"]
    download_url_api = f"{BASE_URL}/internal/market-dumps/{dump_id}/download-url"

    headers = {"Accept": "application/json"}
    if api_key:
        headers["X-api-key"] = api_key

    print(f"\n🔗 尝试 API 获取下载链接: {download_url_api}")
    print("   （注：该端点可能需要浏览器 Cookie，X-api-key 可能不生效）")

    try:
        import urllib.request

        req = urllib.request.Request(download_url_api, headers=headers)
        with urllib.request.urlopen(req, timeout=10) as resp:
            import json

            body = json.loads(resp.read().decode())
    except Exception as e:
        print(f"   ❌ API 调用失败: {e}")
        return None

    code = body.get("code", -1)
    if code != 0:
        print(f"   ❌ API 返回错误: code={code}, message={body.get('message', '')}")
        if code == 2001:
            print("   💡 提示：该端点需要浏览器登录态 Cookie，请用方式二手动下载")
        return None

    # 成功获取下载链接
    download_url = body.get("data", {}).get("url") or body.get("data", {}).get("download_url")
    if not download_url:
        print("   ❌ 响应中未找到下载链接")
        return None

    print(f"   ✅ 获取到预签名链接，开始下载...")

    output_file = output_dir / f"{dump_id}.parquet"
    try:
        urllib.request.urlretrieve(download_url, str(output_file))
        file_size = output_file.stat().st_size
        print(f"   ✅ 下载完成: {output_file} ({file_size / 1024 / 1024:.1f} MB)")
        return output_file
    except Exception as e:
        print(f"   ❌ 下载失败: {e}")
        return None


def manual_download_instructions(kind: str, output_dir: Path) -> Path | None:
    """
    引导用户手动下载 Parquet 文件。

    返回用户放置的文件路径，或 None（用户跳过）。
    """
    dump_id = DUMP_KINDS[kind]["dump_id"]
    download_page = DUMP_KINDS[kind]["download_page"]
    expected_file = output_dir / f"{dump_id}.parquet"

    print(f"""
{'=' * 60}
📋 手动下载步骤（预签名链接 5 分钟有效）:
{'=' * 60}

1. 浏览器打开: {download_page}
2. 用同花顺账号登录
3. 点击页面上的「获取下载链接」按钮（对应「{DUMP_KINDS[kind]['description']}」）
4. 复制弹窗中的预签名链接

5. 在终端执行（替换 <URL>）:

   curl -o "{expected_file}" "<URL>"

   或者直接在浏览器打开链接下载，然后将文件移动到:
   {expected_file}

{'=' * 60}
""")

    # 检查用户是否已放置文件
    if expected_file.exists():
        file_size = expected_file.stat().st_size
        print(f"✅ 检测到已有文件: {expected_file} ({file_size / 1024 / 1024:.1f} MB)")
        return expected_file

    print(f"⏳ 等待文件放置到: {expected_file}")
    print("   放置完成后按 Enter 继续，或输入 'skip' 跳过...")

    while True:
        choice = input("   > ").strip().lower()
        if choice == "skip":
            print("   ⏭️ 跳过下载")
            return None
        if expected_file.exists():
            file_size = expected_file.stat().st_size
            print(f"   ✅ 检测到文件 ({file_size / 1024 / 1024:.1f} MB)")
            return expected_file
        print(f"   尚未检测到文件，请确认路径后按 Enter，或输入 'skip' 跳过...")


def read_and_export(
    parquet_path: Path,
    output_dir: Path,
    kind: str,
    export_csv: bool = False,
    from_date: str | None = None,
    to_date: str | None = None,
    ticker_filter: str | None = None,
    check_duplicates: bool = False,
) -> None:
    """读取 Parquet 文件，打印摘要，并可选导出 CSV。"""
    try:
        import pyarrow.parquet as pq
    except ImportError:
        print("❌ 缺少 pyarrow 依赖，请执行: pip install pyarrow pandas")
        sys.exit(1)

    print(f"\n📖 读取 Parquet: {parquet_path}")

    # 读取 schema 和元信息
    parquet_file = pq.ParquetFile(str(parquet_path))
    schema = parquet_file.schema_arrow
    num_rows = parquet_file.metadata.num_rows
    num_columns = parquet_file.metadata.num_columns

    print(f"   行数: {num_rows:,}")
    print(f"   列数: {num_columns}")
    print(f"   列名: {[f.name for f in schema]}")

    # 自动检测类型
    has_ex_date = any(f.name == "ex_date_ms" for f in schema)
    detected_kind = "adjustment-factors" if has_ex_date else "daily-k"

    print(f"   类型: {'复权因子' if has_ex_date else '日K线'}")

    # 读取数据（可按条件过滤）
    if ticker_filter or from_date or to_date:
        print("\n🔍 应用过滤条件...")
        table = parquet_file.read()
        df = table.to_pandas()

        if ticker_filter:
            ticker_col = "thscode" if "thscode" in df.columns else "ticker"
            mask = df[ticker_col].str.contains(ticker_filter, na=False)
            df = df[mask]
            print(f"   过滤 thscode='{ticker_filter}': {len(df):,} 行")

        date_col = "date_ms" if "date_ms" in df.columns else "ex_date_ms"
        if from_date:
            from_ms = int(datetime.strptime(from_date, "%Y-%m-%d").timestamp() * 1000)
            df = df[df[date_col] >= from_ms]
            print(f"   过滤 from {from_date}: {len(df):,} 行")
        if to_date:
            to_ms = int(datetime.strptime(to_date, "%Y-%m-%d").timestamp() * 1000) + 86400000
            df = df[df[date_col] < to_ms]
            print(f"   过滤 to {to_date}: {len(df):,} 行")
    else:
        # 全部读取
        table = parquet_file.read()
        df = table.to_pandas()

    print(f"\n📊 数据预览（前 5 行）:")
    print(df.head().to_string(max_colwidth=20))
    print(f"\n📊 数据统计:")
    print(df.describe(include="all").to_string(max_colwidth=20))

    # 检查重复
    if check_duplicates:
        if detected_kind == "daily-k":
            dup_cols = ["thscode", "date_ms"]
        else:
            dup_cols = ["thscode", "ex_date_ms"]
        available_cols = [c for c in dup_cols if c in df.columns]
        if len(available_cols) == 2:
            dup_mask = df.duplicated(subset=available_cols, keep=False)
            dup_count = dup_mask.sum()
            if dup_count > 0:
                print(f"\n⚠️  发现 {dup_count} 行重复（键: {available_cols}）")
                print(df[dup_mask].head(10).to_string(max_colwidth=20))
            else:
                print(f"\n✅ 无重复（键: {available_cols}）")
        else:
            print(f"\n⚠️  无法检查重复，缺少必要列: {dup_cols} (实际: {available_cols})")

    # 导出 CSV
    if export_csv:
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        dump_id = DUMP_KINDS[kind]["dump_id"]
        csv_path = output_dir / f"{dump_id}_{timestamp}.csv"
        print(f"\n💾 导出 CSV: {csv_path}")
        df.to_csv(str(csv_path), index=False)
        csv_size = csv_path.stat().st_size
        print(f"   ✅ 导出完成 ({csv_size / 1024 / 1024:.1f} MB, {len(df):,} 行)")


def main():
    parser = argparse.ArgumentParser(
        description="下载 fuyao.aicubes.cn 全市场行情数据并导出 CSV",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s daily-k --export-csv
  %(prog)s adjustment-factors --export-csv
  %(prog)s daily-k --parquet-file ./data.parquet --export-csv
  %(prog)s daily-k --export-csv --ticker 600519
  %(prog)s daily-k --export-csv --from-date 2025-01-01 --to-date 2025-06-30
        """,
    )
    parser.add_argument(
        "kind",
        choices=["daily-k", "adjustment-factors"],
        help="数据类型: daily-k（日K线）或 adjustment-factors（复权因子）",
    )
    parser.add_argument(
        "--output-dir",
        default=DEFAULT_OUTPUT_DIR,
        help=f"输出目录（默认: {DEFAULT_OUTPUT_DIR}）",
    )
    parser.add_argument(
        "--parquet-file",
        help="已有的 Parquet 文件路径（跳过下载步骤）",
    )
    parser.add_argument(
        "--api-key",
        default=os.environ.get("FUYAO_API_KEY"),
        help="API Key（也可通过环境变量 FUYAO_API_KEY 设置）",
    )
    parser.add_argument(
        "--export-csv",
        action="store_true",
        help="导出为 CSV 文件",
    )
    parser.add_argument(
        "--from-date",
        help="过滤起始日期 (YYYY-MM-DD)",
    )
    parser.add_argument(
        "--to-date",
        help="过滤结束日期 (YYYY-MM-DD)",
    )
    parser.add_argument(
        "--ticker",
        help="按 thscode 过滤（如 600519）",
    )
    parser.add_argument(
        "--check-duplicates",
        action="store_true",
        help="检查主键重复",
    )
    parser.add_argument(
        "--skip-download",
        action="store_true",
        help="完全跳过下载，只解析已有文件",
    )

    args = parser.parse_args()

    output_dir = Path(args.output_dir)
    ensure_dir(output_dir)

    # 确定 Parquet 文件路径
    parquet_path = None

    if args.parquet_file:
        # 用户指定了已有文件
        parquet_path = Path(args.parquet_file)
        if not parquet_path.exists():
            print(f"❌ 指定文件不存在: {parquet_path}")
            sys.exit(1)
        print(f"📂 使用已有文件: {parquet_path}")
    elif not args.skip_download:
        # 尝试下载
        api_key = args.api_key
        if api_key:
            print(f"🔑 使用 API Key: {api_key[:8]}...")
        else:
            print("⚠️  未设置 API Key（可通过 --api-key 或环境变量 FUYAO_API_KEY 设置）")

        parquet_path = try_programmatic_download(args.kind, output_dir, api_key)

        if parquet_path is None:
            # 程序化下载失败，引导手动下载
            parquet_path = manual_download_instructions(args.kind, output_dir)
    else:
        # skip_download: 在输出目录找已有文件
        dump_id = DUMP_KINDS[args.kind]["dump_id"]
        candidate = output_dir / f"{dump_id}.parquet"
        if candidate.exists():
            parquet_path = candidate
        else:
            print(f"❌ 跳过下载但未找到已有文件: {candidate}")
            print("   请用 --parquet-file 指定文件路径，或去掉 --skip-download 重新下载")
            sys.exit(1)

    if parquet_path is None:
        print("\n❌ 未获取到数据文件。请重新运行脚本并完成下载。")
        sys.exit(1)

    # 读取和导出
    read_and_export(
        parquet_path=parquet_path,
        output_dir=output_dir,
        kind=args.kind,
        export_csv=args.export_csv,
        from_date=args.from_date,
        to_date=args.to_date,
        ticker_filter=args.ticker,
        check_duplicates=args.check_duplicates,
    )

    print("\n🎉 完成!")
    print(f"   输出目录: {output_dir}")


if __name__ == "__main__":
    main()
