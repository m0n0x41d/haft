"""Reproduce both original init failures using the exact base CLI adapters."""
import json
import subprocess
import tempfile
from pathlib import Path

scratch = Path(__file__).resolve().parent
fixtures = Path(tempfile.mkdtemp(prefix='baseline-', dir=scratch))

def invoke(project, home, binary, host, succeeds):
    result = subprocess.run([str(scratch / binary), str(project), str(home), host, 'apply'], capture_output=True, text=True)
    with (project.parent / 'calls.jsonl').open('a') as output:
        output.write(json.dumps({'binary': binary, 'host': host, 'returncode': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr}) + '\n')
    assert (result.returncode == 0) == succeeds, result.stderr or result.stdout
    return result.stderr

for name in ('fresh', 'both-prior'):
    project = fixtures / name / 'project'
    home = fixtures / name / 'home'
    project.mkdir(parents=True)
    home.mkdir()
    if name == 'both-prior':
        invoke(project, home, 'previous-probe', 'codex', True)
        invoke(project, home, 'previous-probe', 'air', True)
    invoke(project, home, 'base-probe', 'codex', True)
    error = invoke(project, home, 'base-probe', 'air', False)
    assert '.codex/config.toml' in error
    print('PASS original base reproduces blocked codex->air:', name, flush=True)
assert not list(fixtures.rglob('*.db'))
print('PASS baseline probes created no databases; fixtures:', fixtures, flush=True)
