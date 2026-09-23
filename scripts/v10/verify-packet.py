#!/usr/bin/env python3
"""Check the copied execution inputs; never rewrite evidence to hide a mismatch."""
import argparse
import hashlib
import json
from pathlib import Path
p = argparse.ArgumentParser()
p.add_argument('--packet', type=Path, default=Path('.context/haft-v10-2026-09-22'))
a = p.parse_args()
m = json.loads((a.packet / 'EXECUTION-PACKET.json').read_text())
failures = []
for item in m['inputs']:
    b = (a.packet / item['path']).read_bytes()
    if len(b) != item['bytes'] or hashlib.sha256(b).hexdigest() != item['sha256']:
        failures.append(item['path'])
print(json.dumps({'checked': len(m['inputs']), 'mismatches': failures}))
raise SystemExit(bool(failures))
