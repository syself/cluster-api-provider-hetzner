#!/usr/bin/env python3
import sys
from collections import defaultdict


def main():
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <cover.out>", file=sys.stderr)
        sys.exit(1)

    cover_file = sys.argv[1]

    missing_by_file = defaultdict(int)

    try:
        with open(cover_file, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("mode:"):
                    continue

                parts = line.split()
                if len(parts) < 3:
                    # Unexpected line format; skip
                    continue

                file_and_pos = parts[0]
                try:
                    stmts = int(parts[1])
                    count = int(parts[2])
                except ValueError:
                    # Non-integer where we expect ints; skip
                    continue

                # Extract file path from "<file>:<startLine>.<startCol>,<endLine>.<endCol>"
                # Use rsplit to be safe with possible "C:/..." paths.
                file_path, _, _ = file_and_pos.rpartition(":")

                if count == 0:
                    missing_by_file[file_path] += stmts
    except FileNotFoundError:
        print(f"Error: file not found: {cover_file}", file=sys.stderr)
        sys.exit(1)
    except OSError as e:
        print(f"Error reading {cover_file}: {e}", file=sys.stderr)
        sys.exit(1)

    if not missing_by_file:
        print("No missing coverage found (or no valid data).")
        return

    # Sort by missing statements desc, then by file name for stability
    top = sorted(missing_by_file.items(), key=lambda kv: (-kv[1], kv[0]))[:5]

    print(f"{'MISSING':>8}  FILE")
    for file_path, missing in top:
        print(f"{missing:8d}  {file_path}")


if __name__ == "__main__":
    main()
