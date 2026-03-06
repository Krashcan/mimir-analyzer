"""Mimir load test bottleneck analyzer - MCP server wrapper."""

import os
import platform
import subprocess
import sys


def _binary_path():
    """Return the path to the bundled mimir-analyzer binary."""
    pkg_dir = os.path.dirname(os.path.abspath(__file__))
    name = "mimir-analyzer"
    if platform.system() == "Windows":
        name += ".exe"
    return os.path.join(pkg_dir, "bin", name)


def main():
    binary = _binary_path()
    if not os.path.exists(binary):
        print(f"error: mimir-analyzer binary not found at {binary}", file=sys.stderr)
        sys.exit(1)

    # Ensure executable bit is set (ZIP wheels don't preserve it)
    if platform.system() != "Windows":
        st = os.stat(binary)
        if not (st.st_mode & 0o111):
            os.chmod(binary, st.st_mode | 0o111)

    if platform.system() != "Windows":
        os.execvp(binary, [binary] + sys.argv[1:])
    else:
        sys.exit(subprocess.call([binary] + sys.argv[1:]))
