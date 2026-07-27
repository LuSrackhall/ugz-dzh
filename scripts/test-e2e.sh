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
#   7. 自动打开 2025-12 和 2026-06 账本 xlsx
#
# 数据目录：test/e2e/test_data/2025_09/ ~ 2026_06/
# 输出目录：test/e2e/out/（gitignored）
#
# 用法：
#   bash scripts/test-e2e.sh              # 生成 + 测试 + 打开
#   bash scripts/test-e2e.sh --skip-test  # 只生成不测试

set -euo pipefail

cd "$(dirname "$0")/.."
LEDGER="./ledger"
OUT="test/e2e/out"

# 工作树兼容：若 test_data 不存在，从主工作树创建 symlink
TEST_DATA="test/e2e/test_data"
if [ ! -d "$TEST_DATA/2025_10" ]; then
  MAIN_DIR=$(git worktree list | grep "\[main\]" | awk '{print $1}')
  if [ -z "$MAIN_DIR" ] || [ ! -d "$MAIN_DIR/$TEST_DATA/2025_10" ]; then
    # 回退：找第一个有 test_data 的工作树
    while IFS= read -r line; do
      dir=$(echo "$line" | awk '{print $1}')
      if [ -d "$dir/$TEST_DATA/2025_10" ]; then
        MAIN_DIR="$dir"
        break
      fi
    done < <(git worktree list)
  fi
  if [ -n "$MAIN_DIR" ] && [ -d "$MAIN_DIR/$TEST_DATA/2025_10" ]; then
    rm -f "$TEST_DATA"
    ln -sf "$MAIN_DIR/$TEST_DATA" "$TEST_DATA"
    echo "  test_data 已从 $MAIN_DIR 链接到工作树"
  else
    echo "错误：找不到 test_data。请先将测试数据放入主工作树的 $TEST_DATA/ 目录" >&2
    exit 1
  fi
fi

ML_SUPPRESS='[
  "应收款", "应付款", "库存现金", "银行存款",
  "其他应收款", "其他应付款", "其他流动资产",
  "其他流动负债", "其他非流动资产", "其他非流动负债",
  "公益支出", "管理费用", "补助收入", "其他收入",
  "经营收入", "资本", "内部往来", "其他支出", "固定资产", "长期投资"
]'

SKIP_TEST=false
for arg in "$@"; do case "$arg" in --skip-test) SKIP_TEST=true ;; esac; done

echo "=== 1. 编译 ==="
go build -o "$LEDGER" .

echo "=== 2. 初始化（2025-10）==="
rm -rf "$OUT"
"$LEDGER" init -s "2025-10" -o "$OUT"

python3 -c "
import json
with open('$OUT/2025/2025.json') as f:
    cfg = json.load(f)
cfg.setdefault('全局设置', {})['多科目明细账忽略科目'] = $ML_SUPPRESS
with open('$OUT/2025/2025.json', 'w') as f:
    json.dump(cfg, f, ensure_ascii=False, indent=2)
print('  MLSuppressAccounts 已写入')
"

echo "=== 3. 生成 2025-10 ~ 2025-12 ==="
for m in 10 11 12; do
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
# 复制并重命名关键月份，避免 WPS 同名冲突
WT_NAME=$(git rev-parse --abbrev-ref HEAD 2>/dev/null | sed 's/[^a-zA-Z0-9_-]/-/g')
if [ -n "$WT_NAME" ] && [ "$WT_NAME" != "main" ]; then
  for f in "$OUT/2025/2025-12.xlsx" "$OUT/2026/2026-06.xlsx"; do
    if [ -f "$f" ]; then
      base=$(basename "$f")
      cp "$f" "$OUT/$WT_NAME-$base"
      echo "  已复制: $OUT/$WT_NAME-$base"
    fi
  done
fi

find "$OUT" -name "*.xlsx" | sort

# 自动打开 2025-12 和 2026-06 账本，方便快速手动测试
if [ -n "$WT_NAME" ] && [ "$WT_NAME" != "main" ]; then
  for f in "$OUT/$WT_NAME-2025-12.xlsx" "$OUT/$WT_NAME-2026-06.xlsx"; do
    [ -f "$f" ] && open "$f"
  done
else
  for f in "$OUT/2025/2025-12.xlsx" "$OUT/2026/2026-06.xlsx"; do
    [ -f "$f" ] && open "$f"
  done
fi

echo "=== 完成 ==="
