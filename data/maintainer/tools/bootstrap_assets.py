#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
bootstrap_assets.py —— Release 外移资产拉取器（大文件治理，优化项第 8 点）

为什么需要它：
    仓库不跟踪 OCR 模型（det.onnx / rec.onnx / keys.txt，共约 20.4 MiB），
    它们随 GitHub Release 的发布包 zip 派发。开发者新 clone 仓库（或 worktree
    初始化学术环境）后，没有这些文件，识别任务跑不了 —— 本脚本负责把它们
    从 Release 资产中抽回工作区，并逐条做 sha256 校验。

用法：
    python bootstrap_assets.py            # sync：补齐缺失/校验失败的外移资产（幂等）
    python bootstrap_assets.py --verify   # 全部资产基线体检（含 git 跟踪中的大文件）
    python bootstrap_assets.py --list     # 只列清单与本地状态，不下载
    python bootstrap_assets.py --force    # 忽略已有文件重新下载
    python bootstrap_assets.py --tag v26.09.14 --prefix https://ghproxy.net/

设计约束：
    - 纯标准库（urllib / zipfile / hashlib / json），无第三方依赖；
    - 下载临时文件一律 .part 后缀，校验通过才原子改名，中断不留半成品；
    - zip 抽取带路径穿越防护；文件级 sha256 是唯一完整性依据。

清单文件：同目录 assets_manifest.json（随 git 跟踪，表与代码同批审查）。
作者: 2026-10-02 第 8 点优化。历史 Git 提交中的旧大文件不在治理范围内，不重写历史。
"""

import argparse
import hashlib
import json
import os
import sys
import zipfile
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

SCRIPT_DIR = Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parents[2]  # data/maintainer/tools -> 仓根
DEFAULT_MANIFEST = SCRIPT_DIR / "assets_manifest.json"
CACHE_DIR = REPO_ROOT / "cache" / "_bootstrap"
CHUNK = 1 << 20  # 1 MiB

# (状态标识, 说明)
ST_OK = "ok"          # 存在且 sha256 一致
ST_MISS = "missing"  # 本地不存在
ST_BAD = "bad"        # 存在但 sha256 不一致

# ---------- 基础工具 ----------

def human_size(n):
    n = float(n)
    for unit in ("B", "KiB", "MiB", "GiB"):
        if n < 1024 or unit == "GiB":
            return "%.1f %s" % (n, unit)
        n /= 1024
    return "%d B" % n


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(CHUNK), b""):
            h.update(chunk)
    return h.hexdigest()


def local_state(entry):
    """返回 (state, 实际大小或 0, 实际 sha256 或 '')。"""
    p = REPO_ROOT / entry["path"]
    if not p.is_file():
        return ST_MISS, 0, ""
    digest = sha256_file(p)
    size = p.stat().st_size
    return (ST_OK if digest == entry["sha256"] else ST_BAD), size, digest


def load_manifest(path):
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    rel = data.get("release", {})
    for key in ("repo", "tag", "zip_asset"):
        if not rel.get(key):
            die("manifest.release.%s 为空，请先完善 %s" % (key, path))
    return data


def die(msg, code=1):
    print("[x] %s" % msg)
    sys.exit(code)


# ---------- 下载与抽取 ----------

def build_url(rel, asset_name, prefix):
    base = "https://github.com/%s/releases/download/%s/%s" % (
        rel["repo"], rel["tag"], asset_name)
    return prefix + base if prefix else base


def download(url, dest):
    print("    下载 %s" % url)
    req = Request(url, headers={"User-Agent": "BD2MAA-bootstrap_assets/1.0"})
    try:
        with urlopen(req, timeout=60) as resp:
            total = resp.headers.get("Content-Length")
            total = int(total) if total else 0
            done = 0
            with open(dest, "wb") as out:
                while True:
                    chunk = resp.read(CHUNK)
                    if not chunk:
                        break
                    out.write(chunk)
                    done += len(chunk)
                    if total:
                        pct = done * 100 // total
                        print("\r    进度 %3d%% (%s / %s)" % (
                            pct, human_size(done), human_size(total)), end="", flush=True)
                    else:
                        print("\r    已获取 %s" % human_size(done), end="", flush=True)
        print()
    except HTTPError as e:
        if e.code == 404:
            raise RuntimeError(
                "404 资产不存在：%s\n     （该 Release/tag 可能尚未发布，或资产名变了）" % url)
        raise RuntimeError("HTTP %s: %s" % (e.code, url))
    except URLError as e:
        raise RuntimeError(
            "网络不可达：%s\n     可尝试 --prefix 镜像前缀，或设置 HTTPS_PROXY 环境变量" % e.reason)


def _safe_member_path(member_name):
    """防 zip slip：成员路径必须是仓根内的相对路径，不允许 .. / 盘符 / 绝对路径。"""
    norm = os.path.normpath(member_name).replace("\\", "/")
    if norm.startswith("/") or norm.startswith("../") or norm == ".." or ":" in norm:
        return None
    parts = norm.split("/")
    if any(part in ("", "..") for part in parts):
        return None
    return norm


def extract_from_zip(zip_path, want_path, out_path):
    want = want_path.replace("\\", "/")
    with zipfile.ZipFile(zip_path) as zf:
        # zip 内可能带顶层目录，做后缀匹配但要求归一化后精确等于 want
        chosen = None
        for info in zf.infolist():
            if info.is_dir():
                continue
            safe = _safe_member_path(info.filename)
            if safe is None:
                continue
            if safe == want or safe.endswith("/" + want):
                chosen = info
                break
        if chosen is None:
            raise RuntimeError("zip 内找不到 %s（资产包结构可能已调整）" % want)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        tmp = out_path.with_suffix(out_path.suffix + ".part")
        with zf.open(chosen) as src, open(tmp, "wb") as dst:
            shutil_copyfileobj(src, dst)
    return tmp


def shutil_copyfileobj(src, dst):
    while True:
        buf = src.read(CHUNK)
        if not buf:
            break
        dst.write(buf)


def place_file_with_hash(tmp_path, final_path, expect_sha):
    digest = sha256_file(tmp_path)
    if digest != expect_sha:
        tmp_path.unlink(missing_ok=True)
        raise RuntimeError("校验失败（内容与 manifest 不符，已丢弃临时文件）\n"
                           "      期望 %s\n      实得 %s" % (expect_sha, digest))
    final_path.parent.mkdir(parents=True, exist_ok=True)
    os.replace(tmp_path, final_path)


# ---------- 业务流程 ----------

def cmd_list(manifest, args):
    print("Release: %s @ %s" % (manifest["release"]["repo"], manifest["release"]["tag"]))
    print("-" * 78)
    bad = 0
    for e in manifest["assets"]:
        state, size, digest = local_state(e)
        mark = {"ok": "[OK]    ", "missing": "[MISS]  ", "bad": "[BAD]   "}[state]
        if state != ST_OK:
            bad += 1
        print("%s %-38s %-11s 本地 %s" % (
            mark, e["path"], e["source"], human_size(size) if size else "-"))
    print("-" * 78)
    print("%d 项异常（缺失或哈希不符）" % bad)
    return 0


def cmd_verify(manifest, args):
    print("== 全部资产基线体检（tracked + release-zip，不下载） ==")
    bad = 0
    for e in manifest["assets"]:
        state, size, digest = local_state(e)
        if state == ST_OK:
            print("[OK]   %s (%s)" % (e["path"], human_size(size)))
        elif state == ST_MISS:
            bad += 1
            print("[MISS] %s —— %s" % (e["path"],
                  "运行 sync 从 Release 补齐" if e["source"] == "release-zip"
                  else "git 跟踪文件缺失，检查 checkout / worktree junction"))
        else:
            bad += 1
            print("[BAD]  %s 哈希不符" % e["path"])
            print("       期望 %s" % e["sha256"])
            print("       实得 %s" % digest)
    print("-" * 78)
    if bad:
        print("[x] %d 项异常" % bad)
        return 2
    print("[ok] 全部 %d 项与 manifest 一致" % len(manifest["assets"]))
    return 0


def sync_entry(entry, rel, prefix, force, zip_cache):
    path = entry["path"]
    state, size, _ = local_state(entry)
    if state == ST_OK and not force:
        print("[skip] %s 已存在且校验通过 (%s)" % (path, human_size(size)))
        return True
    if state == ST_BAD and not force:
        print("[fix]  %s 已存在但哈希不符，将重新拉取" % path)

    asset_name = entry.get("asset", rel["zip_asset"])
    source = entry.get("source", "release-zip")
    if source != "release-zip":
        print("[hold] %s source=%s，bootstrap 不负责拉取" % (path, source))
        return False

    url = build_url(rel, asset_name, prefix)
    final = REPO_ROOT / path

    if asset_name == rel["zip_asset"]:
        # 大 zip 只下一次，全程复用缓存
        if zip_cache["path"] is None:
            CACHE_DIR.mkdir(parents=True, exist_ok=True)
            zp = CACHE_DIR / asset_name
            if not zp.is_file() or force:
                download(url, zp.with_suffix(".zip.part"))
                os.replace(zp.with_suffix(".zip.part"), zp)
            else:
                print("    复用缓存 %s (%s)" % (zp, human_size(zp.stat().st_size)))
            zip_cache["path"] = zp
        try:
            tmp = extract_from_zip(zip_cache["path"], path, final)
        except RuntimeError as e:
            print("[x]   %s" % e)
            return False
        try:
            place_file_with_hash(tmp, final, entry["sha256"])
        except RuntimeError as e:
            print("[x]   %s" % e)
            return False
    else:
        # 独立单文件资产（为将来细粒度拆分预留）
        final.parent.mkdir(parents=True, exist_ok=True)
        tmp = final.with_suffix(final.suffix + ".part")
        try:
            download(url, tmp)
            place_file_with_hash(tmp, final, entry["sha256"])
        except RuntimeError as e:
            tmp.unlink(missing_ok=True)
            print("[x]   %s" % e)
            return False

    print("[ok]   %s 已补齐 (%s)" % (path, human_size(final.stat().st_size)))
    return True


def cmd_sync(manifest, args):
    rel = dict(manifest["release"])
    if args.tag:
        rel["tag"] = args.tag
    if args.zip_asset:
        rel["zip_asset"] = args.zip_asset
    prefix = args.prefix if args.prefix is not None else rel.get("url_prefix", "")
    targets = [e for e in manifest["assets"] if e.get("source") == "release-zip"]
    print("== sync 外移资产：%s @ %s ==" % (rel["repo"], rel["tag"]))
    print("   共 %d 项外移资产，下载源 %s" % (
        len(targets), rel.get("zip_asset")))
    zip_cache = {"path": None}
    failed = []
    for e in targets:
        if not sync_entry(e, rel, prefix, args.force, zip_cache):
            failed.append(e["path"])
    print("-" * 78)
    if failed:
        print("[x] %d 项失败：%s" % (len(failed), ", ".join(failed)))
        print("    提示：确认 Release 已发布；直连不通加 --prefix 镜像 或 HTTPS_PROXY")
        return 2
    print("[ok] 外移资产全部就位")
    return 0


def main():
    ap = argparse.ArgumentParser(description="BD2MAA Release 外移资产拉取器")
    ap.add_argument("--manifest", default=str(DEFAULT_MANIFEST),
                    help="清单 JSON 路径（默认同目录 assets_manifest.json）")
    ap.add_argument("--verify", action="store_true", help="全部资产基线体检，不下载")
    ap.add_argument("--list", action="store_true", help="只列清单与本地状态")
    ap.add_argument("--force", action="store_true", help="重新下载，忽略已有文件")
    ap.add_argument("--tag", default="", help="覆盖 manifest 中的 release tag")
    ap.add_argument("--zip-asset", default="", dest="zip_asset",
                    help="覆盖 manifest 中的发布包 zip 资产名")
    ap.add_argument("--prefix", default=None,
                    help="下载 URL 镜像前缀，如 https://ghproxy.net/")
    args = ap.parse_args()

    if not Path(args.manifest).is_file():
        die("找不到清单文件：%s" % args.manifest)
    manifest = load_manifest(args.manifest)

    if args.list:
        return cmd_list(manifest, args)
    if args.verify:
        return cmd_verify(manifest, args)
    return cmd_sync(manifest, args)


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        die("中断（已下载的内容在 cache/_bootstrap/，下次运行复用）", 130)
