#!/usr/bin/env bash
# 全量重建：从 JSON + 凭证目录逐月 generate -f 重建全年账本（Change 12）
# 用法:
#   scripts/rebuild.sh <凭证根目录> <输出目录>
# 凭证目录按月组织（两种形态均可）：
#   <凭证根目录>/2025_10/   或   <凭证根目录>/2025/10/
# 从 JSON 的启动月（建账月）逐月重建到当年最新月；-f 级联删除重建。
set -euo pipefail

VROOT="${1:?用法: scripts/rebuild.sh <凭证根目录> <输出目录>}"
OUT="${2:?用法: scripts/rebuild.sh <凭证根目录> <输出目录>}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LEDGER="${LEDGER:-$SCRIPT_DIR/../ledger}"

# 找 JSON（输出目录下的 {year}/{year}.json）
JSON="$(ls "$OUT"/*/*.json 2>/dev/null | head -1 || true)"
if [ -z "$JSON" ]; then
  echo "错误: $OUT 下找不到 {year}/{year}.json"; exit 1
fi
YEAR="$(basename "$JSON" .json)"
START="$(grep -o '"启动月"[[:space:]]*:[[:space:]]*"[^"]*"' "$JSON" | head -1 | sed 's/.*"\([^"]*\)"$/\1/')"
if [ -z "$START" ]; then
  echo "错误: JSON 无'启动月'字段"; exit 1
fi
echo "== 重建 $YEAR 年账本（启动月 ${START}）=="

for m in 01 02 03 04 05 06 07 08 09 10 11 12; do
  MONTH="$YEAR-$m"
  # 只重建 >= 启动月的月份
  if [ "$MONTH" \< "$START" ]; then continue; fi
  # 找凭证目录（两种形态）
  DIR=""
  for cand in "$VROOT/${YEAR}_${m}" "$VROOT/$YEAR/$m"; do
    if [ -d "$cand" ] && ls "$cand"/*.md >/dev/null 2>&1; then DIR="$cand"; break; fi
  done
  if [ -z "$DIR" ]; then
    echo "-- $MONTH: 无凭证目录，跳过"
    continue
  fi
  echo "-- $MONTH: generate -f ($DIR)"
  "$LEDGER" generate -v "$DIR" -o "$OUT" -f
done
echo "== 重建完成 =="
