from pathlib import Path

root = Path(__file__).resolve().parents[1]
target = root / "ALL.md"

parts = []
for path in sorted(root.rglob("*.md")):
    if path.name == "ALL.md":
        continue
    parts.append(f"\n\n---\n\n# FILE: {path.relative_to(root)}\n\n")
    parts.append(path.read_text(encoding="utf-8"))

target.write_text("".join(parts), encoding="utf-8")
print(target)
