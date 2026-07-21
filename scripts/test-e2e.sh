#!/bin/bash
# test-e2e.sh — 一键测试脚本
#
# 功能：
#   1. 编译 ledger
#   2. init 初始化（2025-09 起）
#   3. 顺序生成 2025-09 ~ 2025-12
#   4. year-close 跨年结转（生成 2026-01 含上年结转行）
#   5. 生成 2026-01 ~ 2026-06
#   6. 运行全部测试
#   7. 可选打开 xlsx
#
# 数据目录：test/e2e/test_data/2025_09/ ~ 2026_06/
# 输出目录：test/e2e/out/（gitignored）
#
# 用法：
#   bash scripts/test-e2e.sh              # 生成 + 测试
#   bash scripts/test-e2e.sh --open       # 生成后打开 xlsx
#   bash scripts/test-e2e.sh --skip-test  # 只生成不测试

set -euo pipefail

cd "$(dirname "$0")/.."
LEDGER="./ledger"
OUT="test/e2e/out"

ML_SUPPRESS='[
  "应收款", "应付款", "库存现金", "银行存款",
  "其他应收款", "其他应付款", "其他流动资产",
  "其他流动负债", "其他非流动资产", "其他非流动负债"
]'

OPEN=false
SKIP_TEST=false
for arg in "$@"; do case "$arg" in --open) OPEN=true ;; --skip-test) SKIP_TEST=true ;; esac; done

echo "=== 1. 编译 ==="
go build -o "$LEDGER" .

echo "=== 2. 初始化（2025-09）==="
rm -rf "$OUT"
"$LEDGER" init -s "2025-09" -o "$OUT"

python3 -c "
import json
with open('$OUT/2025/2025.json') as f:
    cfg = json.load(f)
cfg.setdefault('全局设置', {})['多科目明细账忽略科目'] = $ML_SUPPRESS
with open('$OUT/2025/2025.json', 'w') as f:
    json.dump(cfg, f, ensure_ascii=False, indent=2)
print('  MLSuppressAccounts 已写入')
"

echo "=== 3. 生成 2025-09 ~ 2025-12 ==="
for m in 09 10 11 12; do
  echo -n "  2025-$m ... "
  "$LEDGER" generate -v "test/e2e/test_data/2025_$m" -o "$OUT" -f > /dev/null && echo "OK" || {
    echo "FAIL"; "$LEDGER" generate -v "test/e2e/test_data/2025_$m" -o "$OUT" -f; exit 1; }
done

echo "=== 4. 跨年结转 ==="
"$LEDGER" year-close -j "$OUT/2025/2025.json" -o "$OUT"
# 复制已结转的配置到新年度
cp "$OUT/2025/2025.json" "$OUT/2026/2026.json"

echo "=== 5. 生成 2026-01 ~ 2026-06 ==="
for m in 01 02 03 04 05 06; do
  echo -n "  2026-$m ... "
  "$LEDGER" generate -v "test/e2e/test_data/2026_$m" -o "$OUT" -f > /dev/null && echo "OK" || {
    echo "FAIL"; "$LEDGER" generate -v "test/e2e/test_data/2026_$m" -o "$OUT" -f; exit 1; }
done

echo ""
if [ "$SKIP_TEST" = false ]; then
  echo "=== 6. 测试 ==="
  go test ./... -count=1 -timeout 180s
fi

echo ""
find "$OUT" -name "*.xlsx" | sort

if [ "$OPEN" = true ]; then
  LAST=$(find "$OUT" -name "*.xlsx" ! -name "ledger.xlsx" ! -name "balance.xlsx" | sort | tail -1)
  [ -n "$LAST" ] && open "$LAST"
fi

echo "=== 完成 ==="
