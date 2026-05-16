#!/usr/bin/env python3
"""Compatibility launcher for the Go-based Copilot Proxy.

The backend was rewritten in Go. This script preserves the old
`python main.py` habit by delegating to `go run ./cmd/copilot-proxy`.
Release builds should use the native `copilot-proxy` binary directly.
"""

import os
import shutil
import subprocess
import sys


def main() -> int:
    if shutil.which("go") is None:
        print("Go is required for the compatibility launcher.", file=sys.stderr)
        print("Install Go or run a prebuilt copilot-proxy binary.", file=sys.stderr)
        return 1

    root = os.path.dirname(os.path.abspath(__file__))
    return subprocess.call(["go", "run", "./cmd/copilot-proxy", *sys.argv[1:]], cwd=root)


if __name__ == "__main__":
    raise SystemExit(main())
