#!/usr/bin/env bash
# gen-win-test.sh — 在 Mac 上生成 Windows 平台打印版测试账本
#
# 用途：配合 generate --platform 参数，在 Mac 本地生成"Windows 版"打印版，
#       发到 Windows 上用 WPS 肉眼观察表格是否适配（不溢出/不过小/比例合适），
#       然后回来调整 generator/platform_compensate.go 中的补偿系数，迭代至完善。
#
# 前置：test/e2e/out/{year}/{year}.json 已存在（先跑过 bash scripts/test-e2e.sh）
#
# 用法: bash scripts/gen-win-test.sh [YYYY-MM]    默认 2026-06
# 输出: test/e2e/out/win-test/print/{month}.xlsx  （Windows 版打印版，发到 Windows 即可）
set -euo pipefail

MONTH="${1:-2026-06}"
YEAR="${MONTH%%-*}"
M="${MONTH#*-}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LEDGER="$ROOT/ledger"
SRC_JSON="$ROOT/test/e2e/out/$YEAR/$YEAR.json"
VDIR="$ROOT/test/e2e/test_data/${YEAR}_${M}"
OUT="$ROOT/test/e2e/out/win-test"

if [ ! -f "$SRC_JSON" ]; then
  echo "错误: 缺 $SRC_JSON —— 请先运行 bash scripts/test-e2e.sh 生成基础账本" >&2
  exit 1
fi
if [ ! -d "$VDIR" ]; then
  echo "错误: 缺测试凭证 $VDIR" >&2
  exit 1
fi

echo "== 编译 ledger =="
(cd "$ROOT" && go build -o "$LEDGER" .)

echo "== 生成 ${MONTH} Windows 版打印版（--platform windows）=="
rm -rf "$OUT"
mkdir -p "$OUT/$YEAR"
cp "$SRC_JSON" "$OUT/$YEAR/"
"$LEDGER" generate -v "$VDIR" -o "$OUT" --platform windows -f -V

echo
echo "== 完成 =="
echo "Windows 版打印版: $OUT/print/${MONTH}.xlsx"
echo "请将此文件发到 Windows，用 WPS 打开观察（100% 缩放）："
echo "  1. 表格是否适配页面（不溢出、不过小）"
echo "  2. 长宽比例是否合适（接近 1.36）"
echo "  3. 装订边是否合理（正面左/反面右留白）"
echo "回来告诉我调整方向：列宽/行高系数（generator/platform_compensate.go）"
