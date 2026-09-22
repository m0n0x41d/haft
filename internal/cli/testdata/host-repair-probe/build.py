"""Build the host-only planner/publisher probe; never run init core or tests."""
import json
import shutil
import subprocess
import sys
from pathlib import Path

source = Path(__file__).resolve().parent
root = source.parents[3]
scratch = root / '.context/host-probe'
scratch.mkdir(parents=True, exist_ok=True)
(scratch / 'runner').mkdir(exist_ok=True)
for name in ('bridge.go', 'runner/main.go', 'run.py', 'baseline.py'):
    shutil.copyfile(source / name, scratch / name)

replace = {str(root / 'internal/cli/zz_host_repair_probe.go'): str(scratch / 'bridge.go')}
(scratch / 'overlay.json').write_text(json.dumps({'Replace': replace}, indent=2) + '\n')
projection = (root / 'internal/cli/init_host_projection.go').read_text()
current = 'return currentCodexTOMLFragmentWithStartup(context, 20)'
assert projection.count(current) == 1, 'update prior-generation fixture for changed renderer'
previous = projection.replace(current, 'return currentCodexTOMLFragmentWithStartup(context, 10)')
(scratch / 'previous-init-host-projection.go').write_text(previous)
(scratch / 'previous-startup.go').write_text('package cli\nimport "github.com/m0n0x41d/haft/internal/initplanning"\nfunc publicPreviousCodexStartupRecords(projection initplanning.HostAdapterProjection) ([]initplanning.ManagedFragmentRecord,error) { return nil,nil }\n')
prior_replace = dict(replace)
prior_replace[str(root / 'internal/cli/init_host_projection.go')] = str(scratch / 'previous-init-host-projection.go')
prior_replace[str(root / 'internal/cli/init_public_codex_startup.go')] = str(scratch / 'previous-startup.go')
(scratch / 'previous-overlay.json').write_text(json.dumps({'Replace': prior_replace}, indent=2) + '\n')

def build(args):
    command = ['go', 'build'] + args
    print(' '.join(command), flush=True)
    subprocess.run(command, cwd=root, check=True)

build(['-o', '.context/host-probe/haft', './cmd/haft'])
build(['-overlay', '.context/host-probe/overlay.json', '-o', '.context/host-probe/probe', './.context/host-probe/runner'])
build(['-overlay', '.context/host-probe/previous-overlay.json', '-o', '.context/host-probe/previous-probe', './.context/host-probe/runner'])
if '--baseline' in sys.argv[1:]:
    for name in ('init_host_projection.go', 'init_public_codex_startup.go', 'init_public_plan.go'):
        content = subprocess.check_output(['git', 'show', '17d5bbe5:internal/cli/' + name], cwd=root)
        destination = scratch / ('base-' + name)
        destination.write_bytes(content)
        replace[str(root / 'internal/cli' / name)] = str(destination)
    (scratch / 'base-overlay.json').write_text(json.dumps({'Replace': replace}, indent=2) + '\n')
    build(['-overlay', '.context/host-probe/base-overlay.json', '-o', '.context/host-probe/base-probe', './.context/host-probe/runner'])
