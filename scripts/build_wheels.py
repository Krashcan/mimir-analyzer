#!/usr/bin/env python3
"""Build platform-specific wheels for mimir-analyzer.

Cross-compiles the Go binary for each target platform and packages it
into a Python wheel with the correct platform tag.

Usage:
    # Build a single platform (for local testing)
    python scripts/build_wheels.py --version 1.0.0 --platform darwin-arm64

    # Build with explicit platform tag (used in CI)
    python scripts/build_wheels.py --version 1.0.0 --platform linux-amd64 --platform-tag manylinux_2_17_x86_64

    # Build all platforms
    python scripts/build_wheels.py --version 1.0.0
"""

import argparse
import base64
import csv
import hashlib
import io
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import zipfile

PLATFORMS = {
    "linux-amd64": {
        "goos": "linux",
        "goarch": "amd64",
        "tag": "manylinux_2_17_x86_64",
        "ext": "",
    },
    "linux-arm64": {
        "goos": "linux",
        "goarch": "arm64",
        "tag": "manylinux_2_17_aarch64",
        "ext": "",
    },
    "darwin-amd64": {
        "goos": "darwin",
        "goarch": "amd64",
        "tag": "macosx_10_9_x86_64",
        "ext": "",
    },
    "darwin-arm64": {
        "goos": "darwin",
        "goarch": "arm64",
        "tag": "macosx_11_0_arm64",
        "ext": "",
    },
    "windows-amd64": {
        "goos": "windows",
        "goarch": "amd64",
        "tag": "win_amd64",
        "ext": ".exe",
    },
}

PKG_NAME = "mimir_analyzer"
DIST_NAME = "mimir_analyzer"


def go_build(goos, goarch, version, output_path):
    """Cross-compile the Go binary."""
    env = os.environ.copy()
    env["GOOS"] = goos
    env["GOARCH"] = goarch
    env["CGO_ENABLED"] = "0"

    ldflags = f"-s -w -X main.version={version}"
    cmd = ["go", "build", "-ldflags", ldflags, "-o", output_path, "."]

    print(f"  Building for {goos}/{goarch}...")
    subprocess.check_call(cmd, env=env)


def sha256_digest(filepath):
    """Compute SHA256 hash of a file, return base64-urlsafe-nopad digest."""
    h = hashlib.sha256()
    with open(filepath, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return base64.urlsafe_b64encode(h.digest()).rstrip(b"=").decode("ascii")


def build_wheel(version, platform_key, platform_tag_override=None, dist_dir="dist"):
    """Build a single platform wheel."""
    plat = PLATFORMS[platform_key]
    platform_tag = platform_tag_override or plat["tag"]
    binary_ext = plat["ext"]
    binary_name = "mimir-analyzer" + binary_ext

    # Wheel filename components
    wheel_tag = f"py3-none-{platform_tag}"
    wheel_filename = f"{DIST_NAME}-{version}-{wheel_tag}.whl"

    os.makedirs(dist_dir, exist_ok=True)
    wheel_path = os.path.join(dist_dir, wheel_filename)

    with tempfile.TemporaryDirectory() as tmpdir:
        # Build Go binary
        binary_dest = os.path.join(tmpdir, PKG_NAME, "bin", binary_name)
        os.makedirs(os.path.dirname(binary_dest), exist_ok=True)
        go_build(plat["goos"], plat["goarch"], version, binary_dest)

        # Make executable
        if not binary_ext:
            st = os.stat(binary_dest)
            os.chmod(binary_dest, st.st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)

        # Copy Python source files
        src_dir = os.path.join(os.path.dirname(__file__), "..", "python", "src", PKG_NAME)
        for py_file in ("__init__.py", "__main__.py"):
            src = os.path.join(src_dir, py_file)
            dst = os.path.join(tmpdir, PKG_NAME, py_file)
            shutil.copy2(src, dst)

        # Build RECORD entries
        dist_info = f"{DIST_NAME}-{version}.dist-info"
        dist_info_dir = os.path.join(tmpdir, dist_info)
        os.makedirs(dist_info_dir)

        # METADATA
        metadata = f"""Metadata-Version: 2.1
Name: mimir-analyzer
Version: {version}
Summary: Mimir load test bottleneck analyzer - MCP server for Amazon Managed Prometheus
Requires-Python: >=3.8
"""
        metadata_path = os.path.join(dist_info_dir, "METADATA")
        with open(metadata_path, "w") as f:
            f.write(metadata)

        # WHEEL
        wheel_meta = f"""Wheel-Version: 1.0
Generator: build_wheels.py
Root-Is-Purelib: false
Tag: {wheel_tag}
"""
        wheel_meta_path = os.path.join(dist_info_dir, "WHEEL")
        with open(wheel_meta_path, "w") as f:
            f.write(wheel_meta)

        # entry_points.txt
        entry_points = """[console_scripts]
mimir-analyzer = mimir_analyzer:main
"""
        ep_path = os.path.join(dist_info_dir, "entry_points.txt")
        with open(ep_path, "w") as f:
            f.write(entry_points)

        # Collect all files and compute RECORD
        record_lines = []
        for root, _dirs, files in os.walk(tmpdir):
            for fname in files:
                full = os.path.join(root, fname)
                rel = os.path.relpath(full, tmpdir)
                size = os.path.getsize(full)
                digest = sha256_digest(full)
                record_lines.append(f"{rel},sha256={digest},{size}")

        # RECORD itself has no hash
        record_rel = os.path.join(dist_info, "RECORD")
        record_lines.append(f"{record_rel},,")

        record_path = os.path.join(dist_info_dir, "RECORD")
        with open(record_path, "w") as f:
            f.write("\n".join(record_lines) + "\n")

        # Pack into ZIP (wheel)
        with zipfile.ZipFile(wheel_path, "w", zipfile.ZIP_DEFLATED) as zf:
            for root, _dirs, files in os.walk(tmpdir):
                for fname in files:
                    full = os.path.join(root, fname)
                    arcname = os.path.relpath(full, tmpdir)
                    zf.write(full, arcname)

    print(f"  Built: {wheel_path}")
    return wheel_path


def main():
    parser = argparse.ArgumentParser(description="Build mimir-analyzer wheels")
    parser.add_argument("--version", required=True, help="Package version (e.g. 1.0.0)")
    parser.add_argument("--platform", help="Single platform to build (e.g. darwin-arm64)")
    parser.add_argument("--platform-tag", help="Override platform tag (e.g. manylinux_2_17_x86_64)")
    parser.add_argument("--dist-dir", default="dist", help="Output directory (default: dist)")
    args = parser.parse_args()

    if args.platform:
        if args.platform not in PLATFORMS:
            print(f"Unknown platform: {args.platform}", file=sys.stderr)
            print(f"Available: {', '.join(PLATFORMS.keys())}", file=sys.stderr)
            sys.exit(1)
        build_wheel(args.version, args.platform, args.platform_tag, args.dist_dir)
    else:
        for plat_key in PLATFORMS:
            build_wheel(args.version, plat_key, dist_dir=args.dist_dir)


if __name__ == "__main__":
    main()
