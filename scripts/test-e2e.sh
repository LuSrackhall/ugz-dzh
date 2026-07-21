#!/bin/bash
# test-e2e.sh — 一键测试脚本
#
# 功能：
#   1. 编译 ledger
#   2. init 初始化（2025-09 起）
#   3. 顺序生成 2025-09 ~ 2026-06 共 10 个月账本
#   4. 运行全部测试
#   5. 可选打开最后月份的 xlsx
#
# 数据目录：test/e2e/test_data/
#   需提供  2025_09/ ~ 2025_12/ 和 2026_01/ ~ 2026_06/
#   每目录下放当月凭证 .md 文件
#
# 输出目录：test/e2e/out/（gitignored，不会被提交）
#   out/2025/2025.json     ← 配置（含科目树、余额历史）
#   out/2025/2025-09.xlsx  ← 各月累计工作薄
#   out/2026/2026-06.xlsx
#
# 用法：
#   bash scripts/test-e2e.sh          # 生成 + 测试
#   bash scripts/test-e2e.sh --open   # 生成 + 测试 + 打开 xlsx
#   bash scripts/test-e2e.sh --skip-test  # 只生成不测试

set -euo pipefail

cd "$(dirname "$0")/.."
LEDGER="./ledger"
OUT="test/e2e/out"

# 总分类账忽略科目（放入后生成多科目明细账时跳过这些父级科目，防列满）
ML_SUPPRESS='[
  "应收款", "应付款", "库存现金", "银行存款",
  "其他应收款", "其他应付款", "其他流动资产",
  "其他流动负债", "其他非流动资产", "其他非流动负债"
]'

OPEN=false
SKIP_TEST=false
for arg in "$@"; do
  case "$arg" in --open) OPEN=true ;; --skip-test) SKIP_TEST=true ;; esac
done

echo "=== 1. 编译 ==="
go build -o "$LEDGER" .

echo "=== 2. 初始化（2025-09）==="
rm -rf "$OUT"
"$LEDGER" init -s "2025-09" -o "$OUT"

# 写入 ML 忽略科目
python3 -c "
import json
with open('$OUT/2025/2025.json') as f:
    cfg = json.load(f)
cfg.setdefault('全局设置', {})['多科目明细账忽略科目'] = $ML_SUPPRESS
with open('$OUT/2025/2025.json', 'w') as f:
    json.dump(cfg, f, ensure_ascii=False, indent=2)
print('  MLSuppressAccounts 已写入')
"

echo "=== 3. 生成 2025-09 ~ 2026-06 ==="
MONTHS=(
  2025_09 2025_10 2025_11 2025_12
  2026_01 2026_02 2026_03 2026_04 2026_05 2026_06
)
PREV_YEAR=""
for ym in "${MONTHS[@]}"; do
  year="${ym:0:4}"
  mm="${ym:5:2}"
  if [ "$year" != "$PREV_YEAR" ] && [ -n "$PREV_YEAR" ]; then
    mkdir -p "$OUT/$year"
    cp "$OUT/$PREV_YEAR/$PREV_YEAR.json" "$OUT/$year/$year.json"
    echo "  -> 配置复制到 $year/"
  fi
  PREV_YEAR="$year"
  echo -n "  $year-$mm ... "
  "$LEDGER" generate -v "test/e2e/test_data/$ym" -o "$OUT" -f > /dev/null && echo "OK" || {
    echo "FAIL"; "$LEDGER" generate -v "test/e2e/test_data/$ym" -o "$OUT" -f; exit 1; }
done

echo ""
if [ "$SKIP_TEST" = false ]; then
  echo "=== 4. 测试 ==="
  go test ./... -count=1 -timeout 180s
fi

echo ""
echo "=== 输出文件 ==="
find "$OUT" -name "*.xlsx" | sort

if [ "$OPEN" = true ]; then
  LAST=$(find "$OUT" -name "*.xlsx" ! -name "ledger.xlsx" ! -name "balance.xlsx" | sort | tail -1)
  [ -n "$LAST" ] && open "$LAST"
fi

echo "=== 完成 ==="
