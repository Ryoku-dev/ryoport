#!/usr/bin/env python3
"""Give the embedded Space Mono subset stable, non-Nerd Font names."""

import sys
from fontTools.ttLib import TTFont


def main() -> None:
    path, weight = sys.argv[1:]
    font = TTFont(path)
    full = "Space Mono" if weight == "Regular" else f"Space Mono {weight}"
    postscript = f"SpaceMono-{weight}"
    values = {1: "Space Mono", 4: full, 6: postscript, 16: "Space Mono", 17: weight}
    for record in font["name"].names:
        value = values.get(record.nameID)
        if value is None:
            continue
        encoding = record.getEncoding()
        record.string = value.encode(encoding)
    font.save(path, reorderTables=False)


if __name__ == "__main__":
    main()
