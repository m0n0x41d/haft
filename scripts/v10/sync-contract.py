#!/usr/bin/env python3
"""Generate tracked implementation docs from the execution packet, or check drift."""
import argparse
from pathlib import Path
p = argparse.ArgumentParser()
p.add_argument('--check', action='store_true')
p.add_argument('--packet', type=Path, default=Path('.context/haft-v10-2026-09-22'))
a = p.parse_args()
for source, target in [('CONTRACT.md','carriers.md'), ('RUNTIME-CONTRACT.md','runtime.md')]:
    data = (a.packet / source).read_bytes()
    dst = Path('docs/v10') / target
    if a.check:
        if dst.read_bytes() != data:
            raise SystemExit(f'contract drift: {dst}')
    else:
        dst.parent.mkdir(parents=True, exist_ok=True)
        dst.write_bytes(data)
print('Contract mirrors match' if a.check else 'Contract mirrors generated')
