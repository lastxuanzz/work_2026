### input文件： input.xlsx

| 日期 | 上班打卡 | 下班打卡 |
|------|----------|----------|
| 2026-06-26 | 08:29 | 17:56 |
| xxx | xxx | xxx |

### py脚本

```python
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
加班时间统计工具
================

读取包含打卡记录的 Excel（列：日期 / 上班打卡 / 下班打卡），
按考勤规则自动生成带完整公式的统计 Excel。

用法:
    python overtime_calculator.py <输入文件.xlsx> [-o <输出文件.xlsx>]

需人工填写的只有:
    1. 输入文件: 日期 / 上班打卡 / 下班打卡
    2. 输出文件的"实际提交加班起始时间" / "实际提交加班结束时间" 两列
其余全部为 Excel 公式，修改提交时间后自动联动更新。

输入文件格式（第一行为表头）:
    日期        上班打卡   下班打卡
    2026-01-02  08:28      19:06
    2026-01-03  08:30      20:10
    ...

考勤规则:
    - 上班时间 08:30，晚于 08:35 视为缺勤（红色提醒）
    - 下班时间 17:30，18:00 以后算加班
    - 不足 1 小时不算加班（即使 18:59 也记为 0）
    - 19:00 记为 1.0 小时；此后每 6 分钟 = 0.1 小时，不足 6 分钟舍去
    - 每月加班最多 36 小时，超出部分必须调休
    - 周末（周六/周日）加班必须调休
    - 实际提交加班时间晚于 20:00 可获餐补

输出 Excel 布局:
    第 1 行: 表头
    第 2 行: 汇总（最右侧 N~Q 列，随明细自动更新）
    第 3 行起: 每日明细

    明细列:
      A 日期          B 周几(公式)      C 上班时间     D 下班时间
      E 加班时长(公式) F 加班起始(公式)  G 加班结束(公式)
      H 实际提交起始(手填)  I 实际提交结束(手填)
      J 实际加班时长(公式=提交结束-提交开始)
      K 剩余加班起始(公式)  L 剩余加班结束(公式)  M 剩余加班时长(公式)

    汇总列（第 2 行，最右侧）:
      O 加班总时长       =SUM(E3:E..)
      P 实际提交加班时长 =SUM(J3:J..)
      Q 剩余加班时长     =O2-P2

条件格式（红色背景，仅当前单元格）:
    - 上班时间晚于 08:35（迟到，仅 C 列）
    - 实际下班时间晚于 20:00，但实际提交加班结束时间早于 20:00
      （未覆盖餐补门槛，提醒重新计算，仅 J 列）
    - 汇总实际提交加班时长超过 36h（仅 P2 单元格）
"""

import argparse
import datetime
import sys
from pathlib import Path

from openpyxl import Workbook, load_workbook
from openpyxl.formatting.rule import FormulaRule
from openpyxl.styles import Alignment, Border, Font, PatternFill, Side
from openpyxl.utils import get_column_letter

# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------
WORK_START = datetime.time(8, 30)
WORK_START_TOLERANCE = datetime.time(8, 35)
WORK_END = datetime.time(17, 30)
OT_BASE = datetime.time(18, 0)      # 18:00 起算加班
OT_MIN_UNIT = datetime.time(19, 0)  # 不足 1 小时不算
GRAIN = 6                            # 每 6 分钟 = 0.1 小时
MONTH_LIMIT = 36.0                   # 每月加班上限（小时）
MEAL_TIME = datetime.time(20, 0)     # 餐补时间门槛

RED_FILL = PatternFill(start_color="FFFF0000", end_color="FFFF0000", fill_type="solid")
HEADER_FILL = PatternFill(start_color="FF4472C4", end_color="FF4472C4", fill_type="solid")
TOTAL_FILL = PatternFill(start_color="FFD9E2F3", end_color="FFD9E2F3", fill_type="solid")
THIN = Side(style="thin", color="FF000000")
BORDER = Border(left=THIN, right=THIN, top=THIN, bottom=THIN)
CENTER = Alignment(horizontal="center", vertical="center")
HEADER_FONT = Font(bold=True, color="FFFFFFFF")

# 输出明细表头（A~L 共 12 列）
COLUMNS = [
    "日期", "周几", "上班时间", "下班时间",
    "加班时长", "加班起始时间", "加班结束时间",
    "实际提交加班起始时间", "实际提交加班结束时间", "实际加班时长",
    "剩余加班起始时间", "剩余加班结束时间", "剩余加班时长",
]

# 汇总表头（最右侧 O~Q 列，一气呵成，不再留空列）
SUMMARY_HEADERS = ["加班总时长", "实际提交加班时长", "剩余加班时长（需调休）"]
SUMMARY_COLS = [15, 16, 17]  # O, P, Q

FIRST_DATA_ROW = 3               # 明细从第 3 行开始，第 2 行为汇总
M_SPACER = 14                    # N 列作为分隔空白列


# ---------------------------------------------------------------------------
# 工具函数
# ---------------------------------------------------------------------------
def parse_time(value, cell_desc):
    """把 Excel 中的时间值解析为 datetime.time。
    支持: datetime.time / datetime.datetime / 字符串("08:30" / "8:30")
    """
    if isinstance(value, datetime.time):
        return value
    if isinstance(value, datetime.datetime):
        return value.time()
    if isinstance(value, str):
        s = value.strip().replace("：", ":")
        for fmt in ("%H:%M", "%H:%M:%S", "%I:%M%p"):
            try:
                return datetime.datetime.strptime(s, fmt).time()
            except ValueError:
                continue
    raise ValueError(f"{cell_desc} 无法解析为时间: {value!r}")


def parse_date(value, cell_desc):
    """把日期解析为 datetime.date。支持 datetime / date / 字符串。"""
    if isinstance(value, datetime.datetime):
        return value.date()
    if isinstance(value, datetime.date):
        return value
    if isinstance(value, str):
        s = value.strip()
        for fmt in ("%Y-%m-%d", "%Y/%m/%d", "%Y.%m.%d"):
            try:
                return datetime.datetime.strptime(s, fmt).date()
            except ValueError:
                continue
    raise ValueError(f"{cell_desc} 无法解析为日期: {value!r}")


def calc_overtime(off_time):
    """按规则计算加班时长（返回小时数，float）。"""
    if off_time < OT_MIN_UNIT:
        return 0.0
    delta_min = (off_time.hour - 19) * 60 + off_time.minute
    units = delta_min // GRAIN
    return round(1.0 + units * 0.1, 1)


def span_hours(start, end):
    """计算一段区间的时长（小时，向下舍入到 0.1 精度）。"""
    if start is None or end is None:
        return 0.0
    s = datetime.datetime.combine(datetime.date.today(), start)
    e = datetime.datetime.combine(datetime.date.today(), end)
    minutes = int((e - s).total_seconds() // 60)
    if minutes < 0:
        return 0.0
    return round((minutes // GRAIN) * 0.1, 1)


def is_weekend(d):
    return d.weekday() >= 5


# ---------------------------------------------------------------------------
# 读取输入
# ---------------------------------------------------------------------------
def is_missing(value):
    """判断打卡值是否为空/未打卡（None、空字符串、'-'、'--'）。"""
    if value is None:
        return True
    if isinstance(value, str):
        s = value.strip().replace("：", ":")
        return s in ("", "-", "--", "无", "未打卡")
    return False


def load_input(path):
    """读取输入 Excel，返回 [dict, ...]，每行含 date/in/out 与 miss 标记。"""
    wb = load_workbook(path, data_only=True)
    ws = wb.active
    headers = [c.value for c in ws[1]]
    col_idx = {}
    for i, h in enumerate(headers):
        if h is None:
            continue
        h = str(h).strip()
        if "日期" in h:
            col_idx["date"] = i
        elif "上班" in h:
            col_idx["in"] = i
        elif "下班" in h:
            col_idx["out"] = i

    if "date" not in col_idx or "in" not in col_idx or "out" not in col_idx:
        raise ValueError("输入表需要包含 日期 / 上班打卡 / 下班打卡 三列（表头匹配含'日期''上班''下班'的列）")

    rows = []
    for row in ws.iter_rows(min_row=2):
        date_val = row[col_idx["date"]].value
        in_val = row[col_idx["in"]].value
        out_val = row[col_idx["out"]].value
        if date_val is None:
            continue
        d = parse_date(date_val, f"第{row[0].row}行日期")

        # 未打卡识别：若上下班任一为缺失值，标记 miss（不抛异常）
        if is_missing(in_val) and is_missing(out_val):
            rows.append({"date": d, "in": None, "out": None, "miss": True,
                         "miss_weekend": is_weekend(d)})
            continue

        t_in = parse_time(in_val, f"第{row[0].row}行上班打卡")
        t_out = parse_time(out_val, f"第{row[0].row}行下班打卡")
        rows.append({"date": d, "in": t_in, "out": t_out, "miss": False,
                     "miss_weekend": False})
    rows.sort(key=lambda r: r["date"])
    return rows


def process(rows, submitted_map=None):
    """计算每日基础数据与汇总。

    返回:
      details: [dict], 每行含 date/weekend/miss/time_in/time_out/overtime
      summary: dict（供控制台打印；输出 Excel 的汇总为公式）
    """
    details = []
    total_overtime = 0.0    # 加班总时长
    total_submitted = 0.0   # 用户实际提交的加班时长

    for rec in rows:
        d = rec["date"]
        weekend = is_weekend(d)
        miss = rec.get("miss", False)

        if miss:
            overtime = 0.0
            t_in = t_out = None
        else:
            t_in, t_out = rec["in"], rec["out"]
            overtime = calc_overtime(t_out)

        sub_start, sub_end = None, None
        if submitted_map and d in submitted_map:
            sub_start, sub_end = submitted_map[d]
        submitted_overtime = span_hours(sub_start, sub_end) if sub_start else 0.0

        total_overtime = round(total_overtime + overtime, 1)
        total_submitted = round(total_submitted + submitted_overtime, 1)

        details.append({
            "date": d, "weekend": weekend, "miss": miss,
            "time_in": t_in, "time_out": t_out,
            "overtime": overtime,
            "submitted_start": sub_start, "submitted_end": sub_end,
        })

    summary = {
        "加班总时长": total_overtime,
        "实际提交加班时长": total_submitted,
        "剩余加班时长（需调休时长）": round(total_overtime - total_submitted, 1),
    }
    return details, summary


# ---------------------------------------------------------------------------
# 读取用户已填写的提交时间
# ---------------------------------------------------------------------------
def read_submitted(out_path):
    """从已生成的输出文件中读取用户填写的"实际提交加班起始/结束时间"。

    返回: {日期: (起始time, 结束time), ...}
    """
    if not out_path.exists():
        return {}
    wb = load_workbook(out_path, data_only=True)
    ws = wb.active
    headers = [c.value for c in ws[1]]
    ci_date = ci_sub_start = ci_sub_end = None
    for i, h in enumerate(headers):
        h = str(h or "").strip()
        if "日期" in h:
            ci_date = i
        elif "实际提交" in h and ("起始" in h or "开始" in h):
            ci_sub_start = i
        elif "实际提交" in h and "结束" in h:
            ci_sub_end = i
    if ci_date is None or ci_sub_start is None or ci_sub_end is None:
        return {}
    result = {}
    for row in ws.iter_rows(min_row=FIRST_DATA_ROW):
        date_cell = row[ci_date].value
        s_cell = row[ci_sub_start].value
        e_cell = row[ci_sub_end].value
        if date_cell is None:
            continue
        try:
            d = parse_date(date_cell, "输出文件日期列")
        except ValueError:
            continue
        s = parse_time(s_cell, "实际提交起始") if s_cell and str(s_cell).strip() and str(s_cell).strip() != "--" else None
        e = parse_time(e_cell, "实际提交结束") if e_cell and str(e_cell).strip() and str(e_cell).strip() != "--" else None
        if s and e:
            result[d] = (s, e)
    return result


# ---------------------------------------------------------------------------
# 生成输出 Excel（全公式）
# ---------------------------------------------------------------------------
def write_output(details, summary, out_path):
    wb = Workbook()
    ws = wb.active
    ws.title = "加班统计"

    last_row = FIRST_DATA_ROW + len(details) - 1

    # ---- 表头（第 1 行）----
    for ci, col in enumerate(COLUMNS, start=1):
        c = ws.cell(row=1, column=ci, value=col)
        c.font = HEADER_FONT
        c.fill = HEADER_FILL
        c.alignment = CENTER
        c.border = BORDER
    for ci, col in zip(SUMMARY_COLS, SUMMARY_HEADERS):
        c = ws.cell(row=1, column=ci, value=col)
        c.font = HEADER_FONT
        c.fill = HEADER_FILL
        c.alignment = CENTER
        c.border = BORDER

    # ---- 汇总行（第 2 行，最右侧 O~R 列）----
    ws.merge_cells(start_row=2, start_column=1, end_row=2, end_column=13)
    label = ws.cell(row=2, column=1, value="汇总（自动计算）")
    label.font = Font(bold=True)
    label.fill = TOTAL_FILL
    label.alignment = CENTER
    label.border = BORDER
    for col in range(1, 14):
        ws.cell(row=2, column=col).fill = TOTAL_FILL
        ws.cell(row=2, column=col).border = BORDER

    # 汇总公式：
    #   O = 加班总时长       = SUM(E..)   全部原始加班
    #   P = 实际提交加班时长 = SUM(J..)   用户实际提交
    #   Q = 剩余加班时长     = O - P      需调休/未提交部分
    if len(details) > 0:
        o2 = f"=SUM(E{FIRST_DATA_ROW}:E{last_row})"
        p2 = f"=SUM(J{FIRST_DATA_ROW}:J{last_row})"
        q2 = "=O2-P2"
    else:
        o2, p2, q2 = 0.0, 0.0, 0.0
    summary_map = {15: o2, 16: p2, 17: q2}
    for col, val in summary_map.items():
        c = ws.cell(row=2, column=col, value=val)
        c.font = Font(bold=True)
        c.fill = TOTAL_FILL
        c.border = BORDER
        c.alignment = CENTER
        c.number_format = "0.0"

    # ---- 明细行（第 3 行起）----
    for i, row in enumerate(details):
        r = FIRST_DATA_ROW + i
        c = ws.cell(row=r, column=1, value=row["date"])
        c.number_format = "yyyy-mm-dd"

        ws.cell(row=r, column=2,
                value=f'=IF(WEEKDAY(A{r},2)>=6,TEXT(A{r},"aaa")&"(周末)",TEXT(A{r},"aaa"))')

        # 未打卡日：C/D 显示 "-"，E/F/G 等加班公式跳过（直接用 0 /
        if row["miss"]:
            c = ws.cell(row=r, column=3, value="-")
            c.number_format = "@"
            c = ws.cell(row=r, column=4, value="-")
            c.number_format = "@"
            c = ws.cell(row=r, column=5, value=0)
            c.number_format = "0.0"
            ws.cell(row=r, column=6, value="")
            ws.cell(row=r, column=7, value="")
        else:
            c = ws.cell(row=r, column=3, value=row["time_in"])
            c.number_format = "HH:MM"
            c = ws.cell(row=r, column=4, value=row["time_out"])
            c.number_format = "HH:MM"

            c = ws.cell(row=r, column=5,
                        value=f'=IF(D{r}<TIME(19,0,0),0,ROUNDDOWN(ROUND((D{r}-TIME(19,0,0))*1440,10)/6,0)/10+1)')
            c.number_format = "0.0"

            c = ws.cell(row=r, column=6, value=f'=IF(E{r}>0,TIME(18,0,0),"")')
            c.number_format = "HH:MM"
            c = ws.cell(row=r, column=7, value=f'=IF(E{r}>0,D{r},"")')
            c.number_format = "HH:MM"

        c = ws.cell(row=r, column=8, value=row["submitted_start"])
        c.number_format = "HH:MM"
        c = ws.cell(row=r, column=9, value=row["submitted_end"])
        c.number_format = "HH:MM"

        c = ws.cell(row=r, column=10,
                    value=(f'=IF(AND(ISNUMBER(H{r}),ISNUMBER(I{r})),'
                           f'ROUNDDOWN(ROUND((I{r}-H{r})*1440,10)/6,0)/10,'
                           f'IF(AND(H{r}<>"",I{r}<>""),'
                           f'ROUNDDOWN(ROUND((TIMEVALUE(I{r})-TIMEVALUE(H{r}))*1440,10)/6,0)/10,""))'))
        c.number_format = "0.0"

        c = ws.cell(row=r, column=11,
                    value=f'=IF(AND(E{r}>0,OR(J{r}="",J{r}<E{r})),IF(J{r}="",F{r},I{r}),"")')
        c.number_format = "HH:MM"
        c = ws.cell(row=r, column=12, value=f'=IF(K{r}<>"",G{r},"")')
        c.number_format = "HH:MM"

        # M 列：剩余加班时长 = E - J（未提交时为 E，提交足额为 0）
        c = ws.cell(row=r, column=13,
                    value=f'=IF(E{r}>0,IF(J{r}="",E{r},MAX(0,E{r}-J{r})),0)')
        c.number_format = "0.0"

        for col in range(1, 14):
            cell = ws.cell(row=r, column=col)
            cell.alignment = CENTER
            cell.border = BORDER
        # 工作日未打卡：上班时间 C 列标红（缺勤），周末不标
        if row["miss"] and not row["weekend"]:
            ws.cell(row=r, column=3).fill = RED_FILL

    # ---- 条件格式（红色提醒，仅当前单元格）----
    if len(details) > 0:
        # 迟到：仅上班时间 C 列变红（数值时间 且 晚于 8:35）
        ws.conditional_formatting.add(
            f"C{FIRST_DATA_ROW}:C{last_row}",
            FormulaRule(formula=[f'=AND(ISNUMBER($C{FIRST_DATA_ROW}),'
                                 f'$C{FIRST_DATA_ROW}>TIME(8,35,0))'],
                        fill=RED_FILL),
        )
        # 提交未覆盖餐补门槛（实际下班>20:00 但提交结束<20:00）：仅实际加班时长 J 列变红
        ws.conditional_formatting.add(
            f"J{FIRST_DATA_ROW}:J{last_row}",
            FormulaRule(formula=[f'=AND($D{FIRST_DATA_ROW}>TIME(20,0,0),'
                                 f'$I{FIRST_DATA_ROW}<>"",'
                                 f'$I{FIRST_DATA_ROW}<TIME(20,0,0))'],
                        fill=RED_FILL),
        )
        # 实际提交加班时长超过 36h：仅汇总行 P2 单元格变红
        ws.conditional_formatting.add(
            "P2",
            FormulaRule(formula=[f"=$P2>{MONTH_LIMIT}"], fill=RED_FILL),
        )

    # ---- 列宽 / 冻结 ----
    widths = [12, 12, 12, 12, 10, 14, 14, 20, 20, 12, 20, 20, 12, 3, 12, 14, 14]
    for i, w in enumerate(widths, start=1):
        ws.column_dimensions[get_column_letter(i)].width = w
    ws.freeze_panes = f"A{FIRST_DATA_ROW}"

    wb.save(out_path)
    print(f"已生成输出文件: {out_path}")


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------
def main():
    ap = argparse.ArgumentParser(description="加班时间统计工具")
    ap.add_argument("input", help="输入 Excel 文件路径")
    ap.add_argument("-o", "--output", help="输出 Excel 文件路径（默认: input_加班统计.xlsx）")
    args = ap.parse_args()

    in_path = Path(args.input)
    if not in_path.exists():
        sys.exit(f"错误: 找不到输入文件 {in_path}")

    out_path = Path(args.output) if args.output else Path(str(in_path).replace(".xlsx", "_加班统计.xlsx"))

    rows = load_input(in_path)
    print(f"读取到 {len(rows)} 条打卡记录")

    submitted = read_submitted(out_path)
    if submitted:
        print(f"读取到 {len(submitted)} 条用户填写的实际提交加班时间")

    details, summary = process(rows, submitted_map=submitted)
    write_output(details, summary, out_path)

    print("系统核算汇总:")
    for k, v in summary.items():
        print(f"  {k}: {v:g} 小时")


if __name__ == "__main__":
    main()

```
