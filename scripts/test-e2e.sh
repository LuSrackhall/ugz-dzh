#!/bin/bash
# test-e2e.sh — 一键测试脚本
#
# 功能：
#   1. 编译 ledger 二进制
#   2. 初始化空白配置
#   3. 顺序生成指定月份的完整账本
#   4. 运行全部单元测试 + e2e 测试
#   5. 可选：打开生成的 xlsx 查看效果
#
# 前置条件：
#   - Go 1.21+
#   - 测试数据目录: test/e2e/test_data/2026_01/ ~ 2026_12/
#     (每个目录下存放当月凭证 .md 文件)
#   - 配置文件在首次 init 后自动生成于输出目录
#
# 用法：
#   ./scripts/test-e2e.sh              # 默认生成 1~6 月
#   ./scripts/test-e2e.sh 01 04 06      # 只生成指定月份（01,04,06）
#   ./scripts/test-e2e.sh --open        # 生成后打开 xlsx
#   ./scripts/test-e2e.sh --skip-test   # 跳过测试，只生成文件
#   ./scripts/test-e2e.sh --help        # 显示帮助
#
# 输出目录：
#   test/e2e/out/{年份}/
#     {年份}-{月份}.xlsx     ← 各月累计工作薄
#     {年份}.json           ← 科目余额配置
#     ledger.csv/xlsx       ← 分录汇总
#     balance.csv/xlsx      ← 余额汇总
#
# 注意事项：
#   gitignore 规则屏蔽 test/e2e/out/*，生成的文件不会被提交。
#   如需修改 MLSuppressAccounts 等配置，生成后手动编辑 json 再重新运行。

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

LEDGER_BIN="./ledger"
OUTPUT="test/e2e/out"
TEST_DATA="test/e2e/test_data"
MONTHS=()
OPEN_XLSX=false
SKIP_TEST=false
START_MONTH="2026-01"

# 配置：多科目明细账忽略科目（防止 14 列撑满报错）
ML_SUPPRESS='[
      "应收款",
      "应付款",
      "库存现金",
      "银行存款",
      "其他应收款",
      "其他应付款",
      "其他流动资产",
      "其他流动负债",
      "其他非流动资产",
      "其他非流动负债"
    ]'

show_help() {
  sed -n '3,26p' "$0"
  exit 0
}

# 解析参数
while [[ $# -gt 0 ]]; do
  case "$1" in
    --help) show_help ;;
    --open) OPEN_XLSX=true; shift ;;
    --skip-test) SKIP_TEST=true; shift ;;
    [0-9][0-9]) MONTHS+=("$1"); shift ;;
    *) echo "未知参数: $1"; exit 1 ;;
  esac
done

if [[ ${#MONTHS[@]} -eq 0 ]]; then
  MONTHS=("01" "02" "03" "04" "05" "06")
fi

echo "============================================"
echo "  手工账一键测试"
echo "============================================"
echo "月份: ${MONTHS[*]}"
echo "输出: $OUTPUT"
echo ""

# 1. 编译
echo ">>> 编译 ledger..."
go build -o "$LEDGER_BIN" .
echo "    OK"

# 2. 清理输出
echo ">>> 清理输出目录..."
rm -rf "$OUTPUT"

# 3. Init
echo ">>> 初始化配置..."
"$LEDGER_BIN" init -s "$START_MONTH" -o "$OUTPUT"
echo "    OK"

# 4. 写入 MLSuppressAccounts（防止明细列满报错）
JSON_FILE="$OUTPUT/2026/2026.json"
if [ -f "$JSON_FILE" ]; then
  python3 -c "
import json
with open('$JSON_FILE', 'r') as f:
    cfg = json.load(f)
if not cfg.get('全局设置', {}).get('多科目明细账忽略科目'):
    cfg.setdefault('全局设置', {})['多科目明细账忽略科目'] = $ML_SUPPRESS
    with open('$JSON_FILE', 'w') as f:
        json.dump(cfg, f, ensure_ascii=False, indent=2)
    print('    已写入 MLSuppressAccounts')
else:
    print('    MLSuppressAccounts 已存在，跳过')
"
fi

# 5. 逐月生成
echo ">>> 生成账本..."
for m in "${MONTHS[@]}"; do
  DATA_DIR="$TEST_DATA/2026_$m"
  if [ ! -d "$DATA_DIR" ]; then
    echo "    跳过 2026-$m（目录不存在）"
    continue
  fi
  echo -n "    2026-$m ... "
  if "$LEDGER_BIN" generate -v "$DATA_DIR" -o "$OUTPUT" -f > /dev/null 2>&1; then
    echo "OK"
  else
    echo "FAIL"
    "$LEDGER_BIN" generate -v "$DATA_DIR" -o "$OUTPUT" -f
    exit 1
  fi
done

echo ""

# 6. 测试
if [ "$SKIP_TEST" = false ]; then
  echo ">>> 运行测试..."
  go test ./... -count=1 -timeout 180s 2>&1
  echo ""
else
  echo ">>> 跳过测试"
fi

# 7. 文件列表
echo "============================================"
echo "  输出文件:"
find "$OUTPUT" -type f -name "*.xlsx" | sort
echo ""

# 8. 打开
if [ "$OPEN_XLSX" = true ]; then
  LAST_XLSX=$(find "$OUTPUT" -name "*.xlsx" ! -name "ledger.xlsx" ! -name "balance.xlsx" | sort | tail -1)
  if [ -n "$LAST_XLSX" ]; then
    echo ">>> 打开 $LAST_XLSX"
    open "$LAST_XLSX"
  fi
fi

echo "============================================"
echo "  完成!"
echo "============================================"
