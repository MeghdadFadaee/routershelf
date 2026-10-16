#!/usr/bin/env python3
"""Check public source paths, local Markdown links, and metadata references.

Reads Git-visible files only; ignored private configuration and outputs are never
opened. This is a hygiene check, not a substitute for reviewing secrets or a
release archive. Run from any directory within the checkout.
"""
import json
from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit


def main():
    root = Path(subprocess.check_output(
        ["git", "rev-parse", "--show-toplevel"], text=True).strip())
    names = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=root).decode().split("\0")
    errors = []
    for name in sorted(set(names) - {""}):
        path = root / name
        if path.is_symlink():
            errors.append(f"{name}: symlinks are not supported in public source checks")
            continue
        if not path.is_file():  # Deletions and renames not yet staged.
            continue
        parts = Path(name).parts
        base = path.name
        private = (
            any(part in {"dist", "outputs", ".idea", "__pycache__", "certificates"}
                for part in parts)
            or (base.startswith(".env") and name != ".env.example")
            or (base.startswith(".smb.") and name != ".smb.example.json")
            or base in {"smb.json", "config.sh", "supervisor.private.sh"}
            or path.suffix in {".key", ".pem", ".pcap", ".backup", ".log", ".pid"}
        )
        if private:
            errors.append(f"{name}: private/generated material in public source set")
            continue
        if path.suffix != ".md":
            continue
        # Inline Markdown links; skip web links, anchors, and fenced examples.
        content = re.sub(r"```.*?```", "", path.read_text(), flags=re.S)
        for match in re.finditer(r"\]\((<[^>]+>|[^\s)]+)\)", content):
            target = match[1].strip("<>")
            url = urlsplit(target)
            if url.scheme or url.netloc or not url.path:
                continue
            destination = (path.parent / unquote(url.path)).resolve()
            if not destination.is_relative_to(root) or not destination.exists():
                errors.append(f"{name}: broken local link {target}")
    if errors:
        raise SystemExit("\n".join(errors))
    metadata = json.loads((root / "project.json").read_text())
    for service in metadata["services"].values():
        for key in ("entrypoint", "configuration", "documentation"):
            if key in service and not (root / service[key]).exists():
                errors.append(f"project.json: missing {key} {service[key]}")
    if errors:
        raise SystemExit("\n".join(errors))
    print("Public source paths, local documentation links and service metadata valid")


if __name__ == "__main__":
    main()
