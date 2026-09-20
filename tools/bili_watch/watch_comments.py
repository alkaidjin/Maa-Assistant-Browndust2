#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
B站视频评论区监控（MABD2 / BD2MAA 观众反馈）

用途
----
定时抓取指定视频的评论区，只把**新增**评论挑出来，并按「疑似使用问题 / 功能建议 /
其他」自动分组，输出一份 markdown 报告，供作者快速回复观众。

技术要点
--------
- 视频信息：/x/web-interface/view?bvid=...
- 评论列表：/x/v2/reply/wbi/main（需要 wbi 签名；mode=3 = 时间序，游标翻页）
  * 旧接口 /x/v2/reply 只返回热评前几条且时间序失效，不要用。
  * wbi 签名：nav 接口拿 img_key/sub_key（未登录也返回），按官方重排表取 32 位
    mixin_key，参数排序拼接后 md5 得 w_rid；key 每日轮换，本脚本按日期缓存。
- 匿名即可读取（实测通过）。若提供 SESSDATA 会更稳，见 --sessdata。
- 状态文件记录已见 rpid，二次运行只报新增；子评论用于判断 UP 是否已回复过。

用法
----
    python tools/bili_watch/watch_comments.py                 # 用下面 DEFAULT_BVID
    python tools/bili_watch/watch_comments.py --bvid BVxxxx   # 指定视频
    python tools/bili_watch/watch_comments.py --full          # 忽略状态，全量重报
    python tools/bili_watch/watch_comments.py --pages 5       # 最多翻 5 页
    python tools/bili_watch/watch_comments.py --sessdata XXX   # 带登录态

输出
----
    cache/bili_watch/state.json          已见评论 rpid 与首次见到时间
    cache/bili_watch/snapshot.json       本次全量快照（便于程序化分析）
    cache/bili_watch/新评论-<时间>.md     本次新增评论报告（无新增时不生成）
"""

import argparse
import hashlib
import json
import os
import re
import ssl
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone, timedelta

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

DEFAULT_BVID = "BV1PieQ6gE8v"     # 【MABD2】挂机自动一键日常！设置与详解！
STATE_DIR = os.path.join("cache", "bili_watch")

UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36")
API = "https://api.bilibili.com"
_CTX = ssl._create_unverified_context()

# wbi 的 mixin key 重排表（官方固定值）
MIXIN_KEY_ENC_TAB = [
    46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43, 5, 49,
    33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16, 24, 55, 40, 61,
    26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36,
    20, 34, 44, 52,
]

# ---------------------------------------------------------------- 分类规则

# 强信号：明确的功能异常 / 求助
PROBLEM_STRONG = [
    "报错", "错误", "失败", "卡住", "卡死", "卡在", "闪退", "崩溃", "打不开",
    "用不了", "不能用", "无法", "不触发", "没触发", "没反应", "没有反应", "无反应",
    "不生效", "没生效", "没效果", "不工作", "异常", "黑屏", "白屏", "卡顿", "死循环",
    "脱机", "断连", "连不上", "识别不了", "识别不到", "找不到", "闪一下", "开不了",
    "进不去", "进不了", "过不去", "对不上", "没出现", "不出现", "不动", "动不了",
    "乱点", "点错", "停止任务", "直接停止", "不会进入", "没进入", "没进去", "停在",
    "没开始", "空转", "没成功", "不成功", "没搞定", "百分之0", "0%",
]
# 疑问信号
PROBLEM_ASK = [
    "求助", "请问", "请教", "怎么办", "怎么解决", "为啥", "为什么", "怎么回事",
    "什么原因", "能不能帮我", "帮我看看", "是不是", "有没有人", "怎么弄", "如何",
]
# 功能建议 / 需求
SUGGEST = [
    "希望", "建议", "能不能加", "可不可以加", "求加", "求个", "求一个", "想要", "加上",
    "增加", "新增", "支持一下", "可否", "可以考虑", "能不能支持", "加个", "添加",
    "优化", "改进", "蹲", "期待",
]
# 明显不是问题的（纯互动）
PRAISE = [
    "感谢", "谢谢", "好用", "三连", "已三连", "大佬", "厉害", "牛", "支持", "顶",
    "点赞", "哈哈", "太强", "神器", "nb", "牛批", "收藏", "沙发", "好耶",
]


def http_json(url, referer="https://www.bilibili.com/", sessdata=None, retries=3):
    headers = {
        "User-Agent": UA,
        "Referer": referer,
        "Accept": "application/json, text/plain, */*",
        "Accept-Language": "zh-CN,zh;q=0.9",
    }
    if sessdata:
        headers["Cookie"] = "SESSDATA=" + sessdata
    last = None
    for i in range(retries):
        try:
            req = urllib.request.Request(url, headers=headers)
            with urllib.request.urlopen(req, timeout=30, context=_CTX) as r:
                return json.loads(r.read().decode("utf-8", "replace"))
        except Exception as e:                    # noqa: BLE001 - 网络层统一重试
            last = e
            time.sleep(1.5 * (i + 1))
    return {"code": None, "message": "request failed: %s" % last}


def get_video_info(bvid, sessdata=None):
    d = http_json("%s/x/web-interface/view?bvid=%s" % (API, bvid), sessdata=sessdata)
    if d.get("code") != 0:
        raise SystemExit("取视频信息失败：code=%s msg=%s" % (d.get("code"), d.get("message")))
    v = d["data"]
    return {
        "aid": v["aid"],
        "bvid": v["bvid"],
        "title": v.get("title"),
        "owner": (v.get("owner") or {}).get("name"),
        "owner_mid": (v.get("owner") or {}).get("mid"),
        "reply_count": (v.get("stat") or {}).get("reply"),
    }


_wbi_cache = {"day": None, "key": None}


def get_wbi_key(sessdata=None):
    today = datetime.now().strftime("%Y%m%d")
    if _wbi_cache["day"] == today and _wbi_cache["key"]:
        return _wbi_cache["key"]
    d = http_json("%s/x/web-interface/nav" % API, sessdata=sessdata)
    wi = (d.get("data") or {}).get("wbi_img") or {}
    img = os.path.splitext(os.path.basename(wi.get("img_url") or ""))[0]
    sub = os.path.splitext(os.path.basename(wi.get("sub_url") or ""))[0]
    if not img or not sub:
        raise SystemExit("取 wbi key 失败（nav 未返回 wbi_img）")
    raw = img + sub
    key = "".join(raw[i] for i in MIXIN_KEY_ENC_TAB)[:32]
    _wbi_cache.update(day=today, key=key)
    return key


def signed(url_path, params, sessdata=None):
    key = get_wbi_key(sessdata=sessdata)
    p = dict(params)
    p["wts"] = int(time.time())
    q = urllib.parse.urlencode(sorted(p.items()))
    w_rid = hashlib.md5((q + key).encode()).hexdigest()
    return "%s%s?%s&w_rid=%s" % (API, url_path, q, w_rid)


def simplify_reply(r):
    m = r.get("member") or {}
    c = r.get("content") or {}
    subs = []
    for s in (r.get("replies") or [])[:3]:
        sm = s.get("member") or {}
        subs.append({
            "rpid": s.get("rpid"),
            "uname": sm.get("uname"),
            "mid": sm.get("mid"),
            "message": (s.get("content") or {}).get("message", ""),
        })
    return {
        "rpid": r.get("rpid"),
        "root": r.get("root"),
        "parent": r.get("parent"),
        "ctime": r.get("ctime"),
        "uname": m.get("uname"),
        "mid": m.get("mid"),
        "message": c.get("message", ""),
        "like": r.get("like", 0),
        "rcount": r.get("rcount", 0),
        "is_up_top": bool(c.get("is_up_top") or r.get("is_up_top")),
        "children": subs,
    }


def fetch_comments(aid, max_pages=10, sessdata=None):
    """时间序（mode=3）抓取主评论，游标翻页。

    注意：接口的 cursor.all_count 统计的是**含楼中楼**的评论总数，而 replies 只返回
    主评论；实测同一视频 all_count=26 时主评论仅 14 条。另外 next=0 与 next=1 会返回
    同一页，所以除游标外还要比对 rpid 序列，一旦与上一页相同即停止。
    """
    out, seen_pages = [], set()
    prev_rpids = None
    nxt = 0
    for _ in range(max_pages):
        url = signed("/x/v2/reply/wbi/main", {
            "oid": aid, "type": 1, "mode": 3, "next": nxt, "ps": 20,
            "plat": 1, "web_location": 1315875,
        }, sessdata=sessdata)
        d = http_json(url, referer="https://www.bilibili.com/video/%s" % DEFAULT_BVID,
                      sessdata=sessdata)
        if d.get("code") != 0:
            print("  ! 拉取失败 code=%s msg=%s" % (d.get("code"), d.get("message")))
            break
        data = d.get("data") or {}
        reps = data.get("replies") or []
        cur_rpids = tuple(r.get("rpid") for r in reps)
        if not reps or cur_rpids == prev_rpids:
            break
        prev_rpids = cur_rpids
        for r in reps:
            out.append(simplify_reply(r))

        cur = data.get("cursor") or {}
        if cur.get("is_end"):
            break
        nxt_new = cur.get("next")
        if nxt_new is None or nxt_new == nxt or nxt_new in seen_pages:
            break
        seen_pages.add(nxt)
        nxt = nxt_new
        time.sleep(0.8)          # 温和限速，避免触发风控

    # 按 rpid 去重（置顶评论可能同时出现在首页与热评位置）
    uniq, seen_rpid = [], set()
    for c in out:
        if c["rpid"] in seen_rpid:
            continue
        seen_rpid.add(c["rpid"])
        uniq.append(c)
    return uniq


def classify(text, uname=None, owner=None):
    t = (text or "").strip()
    low = t.lower()
    score, reasons = 0, []
    hits_strong = [w for w in PROBLEM_STRONG if w in t]
    hits_ask = [w for w in PROBLEM_ASK if w in t]
    hits_sug = [w for w in SUGGEST if w in t]
    hits_praise = [w for w in PRAISE if w in t or w in low]
    tail = t.rstrip("！!。.~～ ")
    has_q = ("?" in t) or ("？" in t) or (len(t) >= 5 and tail[-1:] in ("吗", "呢"))

    if hits_strong:
        score += 4 + min(len(hits_strong) - 1, 3)
        reasons.append("异常词：" + "、".join(hits_strong[:4]))
    if hits_ask:
        score += 2 * len(hits_ask)
        reasons.append("求助/追问：" + "、".join(hits_ask[:3]))
    if has_q:
        score += 2
        reasons.append("疑问句式")
    if hits_sug:
        score += 2
        reasons.append("功能建议：" + "、".join(hits_sug[:3]))
    if hits_praise:
        reasons.append("含互动词")
        if not (hits_strong or hits_sug or has_q or hits_ask):
            score -= 2

    # 判定顺序 = 对作者的行动价值：功能异常 > 功能建议 > 提问 > 其他
    if hits_strong or score >= 4:
        kind = "疑似使用问题"
    elif hits_sug:
        kind = "功能建议"
    elif has_q or hits_ask:
        kind = "疑似提问/需求"
    else:
        kind = "其他"
    return kind, score, reasons


def load_json(path, default):
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return default


def save_json(path, obj):
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(obj, f, ensure_ascii=False, indent=2)


def fmt_time(ts):
    if not ts:
        return "?"
    return datetime.fromtimestamp(ts, tz=timezone(timedelta(hours=8))).strftime("%Y-%m-%d %H:%M")


def up_replied(c, owner_mid):
    return any(str(s.get("mid")) == str(owner_mid) for s in (c.get("children") or []))


def render_report(video, new_items, all_items, owner_mid, generated_at):
    lines = []
    lines.append("# B站评论区新增反馈 — %s" % video["title"])
    lines.append("")
    lines.append("- 视频：https://www.bilibili.com/video/%s" % video["bvid"])
    lines.append("- UP主：%s（mid %s）" % (video["owner"], owner_mid))
    lines.append("- 视频页显示评论数：%s 条（**含楼中楼**）｜抓到主评论：%d 条｜本次新增：%d 条"
                 % (video["reply_count"], len(all_items), len(new_items)))
    lines.append("- 抓取时间：%s" % generated_at)
    lines.append("")

    buckets = {"疑似使用问题": [], "疑似提问/需求": [], "功能建议": [], "其他": []}
    for c in new_items:
        kind, score, reasons = classify(c["message"], c["uname"], video["owner"])
        c["_kind"], c["_score"], c["_reasons"] = kind, score, reasons
        c["_up_replied"] = up_replied(c, owner_mid)
        buckets.setdefault(kind, []).append(c)

    order = ["疑似使用问题", "功能建议", "疑似提问/需求", "其他"]
    for kind in order:
        items = sorted(buckets.get(kind, []), key=lambda x: -x["_score"])
        if not items:
            continue
        lines.append("## %s（%d 条）" % (kind, len(items)))
        lines.append("")
        for i, c in enumerate(items, 1):
            replied = "已回复" if c["_up_replied"] else "**未回复**"
            lines.append("### %d. %s ｜ %s ｜ 赞 %s ｜ %s"
                         % (i, c["uname"], fmt_time(c["ctime"]), c["like"], replied))
            lines.append("")
            lines.append("> " + c["message"].replace("\n", "\n> "))
            lines.append("")
            lines.append("- 命中：%s｜rpid `%s`" % ("；".join(c["_reasons"]) or "无", c["rpid"]))
            if c["children"]:
                lines.append("- 已有回复：")
                for s in c["children"]:
                    lines.append("  - %s：%s" % (s["uname"], s["message"][:120]))
            lines.append("")
    return "\n".join(lines)


def main():
    ap = argparse.ArgumentParser(description="B站评论区监控")
    ap.add_argument("--bvid", default=DEFAULT_BVID)
    ap.add_argument("--pages", type=int, default=10, help="最多翻页数（每页约 20 条）")
    ap.add_argument("--full", action="store_true", help="忽略状态文件，全量视为新增")
    ap.add_argument("--sessdata", default=os.environ.get("BILI_SESSDATA"),
                    help="可选：登录态 SESSDATA，匿名也能用")
    ap.add_argument("--state-dir", default=STATE_DIR)
    args = ap.parse_args()

    os.makedirs(args.state_dir, exist_ok=True)
    state_path = os.path.join(args.state_dir, "state.json")
    state = load_json(state_path, {"seen": {}, "bvid": args.bvid})

    video = get_video_info(args.bvid, sessdata=args.sessdata)
    print("视频：%s（aid %s）｜评论 %s 条" % (video["title"], video["aid"], video["reply_count"]))

    comments = fetch_comments(video["aid"], max_pages=args.pages, sessdata=args.sessdata)
    print("抓到主评论 %d 条" % len(comments))

    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    seen = {} if args.full else (state.get("seen") or {})
    new_items = [c for c in comments if str(c["rpid"]) not in seen]
    for c in comments:
        seen.setdefault(str(c["rpid"]), now)
        seen[str(c["rpid"])] = seen[str(c["rpid"])]

    save_json(state_path, {"bvid": args.bvid, "last_run": now, "seen": seen})
    save_json(os.path.join(args.state_dir, "snapshot.json"),
              {"video": video, "generated_at": now, "comments": comments})

    if not new_items:
        print("没有新增评论。")
        return 0

    new_items.sort(key=lambda x: x["ctime"] or 0, reverse=True)
    md = render_report(video, new_items, comments, video["owner_mid"], now)
    tag = datetime.now().strftime("%Y%m%d-%H%M")
    out = os.path.join(args.state_dir, "新评论-%s.md" % tag)
    with open(out, "w", encoding="utf-8") as f:
        f.write(md)

    kinds = {}
    for c in new_items:
        k, _, _ = classify(c["message"])
        kinds[k] = kinds.get(k, 0) + 1
    print("新增 %d 条：%s" % (len(new_items), "，".join("%s %d" % (k, v) for k, v in kinds.items())))
    print("报告：%s" % out)
    return 0


if __name__ == "__main__":
    sys.exit(main())
