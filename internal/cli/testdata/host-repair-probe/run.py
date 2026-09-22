import hashlib
import json
import os
import subprocess
import tempfile
from pathlib import Path

scratch = Path(__file__).resolve().parent
fixtures = Path(tempfile.mkdtemp(prefix='fixtures-', dir=scratch))
environment = {name: os.environ.get(name) for name in ('HOME', 'CODEX_HOME')}
custom = b'\n[mcp_servers.haft.tools.haft_method]\napproval_mode = "prompt"\n\n[mcp_servers.foreign]\ncommand = "foreign-server"\n'
old_config = b'[mcp_servers.haft]\ncommand = "haft"\nargs = ["serve"]\nstartup_timeout_sec = 10\ntool_timeout_sec = 60\n\n[mcp_servers.haft.env]\nHAFT_PROJECT_ROOT = "."\nHAFT_EXPECTED_PROJECT_ID = "qnt_e3149c17"\n'

def fixture(name):
    root = fixtures / name
    project = root / 'project'
    home = root / 'home'
    project.mkdir(parents=True)
    home.mkdir()
    return project, home

def write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)

def snapshot(root):
    return {str(path.relative_to(root)): (hashlib.sha256(path.read_bytes()).hexdigest(), path.stat().st_mtime_ns) for path in root.rglob('*') if path.is_file()}

def state(project, home):
    return snapshot(project), snapshot(home)

def invoke(project, home, host, mode='apply', binary='probe', succeeds=True):
    other = 'air' if host == 'codex' else 'codex'
    other_manifest = project / '.haft/host-installations' / (other + '.project.json')
    preserved = (other_manifest.read_bytes(), other_manifest.stat().st_mtime_ns) if other_manifest.exists() else None
    result = subprocess.run([str(scratch / binary), str(project), str(home), host, mode], capture_output=True, text=True)
    log = project.parent / 'calls.jsonl'
    with log.open('a') as output:
        output.write(json.dumps({'binary': binary, 'host': host, 'mode': mode, 'returncode': result.returncode, 'stdout': result.stdout, 'stderr': result.stderr}) + '\n')
    assert (result.returncode == 0) == succeeds, result.stderr or result.stdout
    if preserved is not None:
        assert (other_manifest.read_bytes(), other_manifest.stat().st_mtime_ns) == preserved, 'unselected host manifest changed'
    return json.loads(result.stdout) if succeeds else result.stderr

def repeat(project, home, order):
    before = state(project, home)
    for host in order:
        result = invoke(project, home, host)
        assert set(result.values()) == {'already_current'}, result
    assert state(project, home) == before, 'repeat changed bytes or mtimes'

def assert_config(project):
    content = (project / '.codex/config.toml').read_bytes()
    assert b'startup_timeout_sec = 20\n' in content
    assert custom in content
    assert b'required =' not in content
    return content

for order in [('codex', 'air'), ('air', 'codex')]:
    label = '-'.join(order)
    for legacy in (False, True):
        project, home = fixture(label + ('-legacy-unowned' if legacy else '-fresh'))
        config_path = project / '.codex/config.toml'
        if legacy:
            write(config_path, old_config + custom)
        for host in order:
            result = invoke(project, home, host)
            assert set(result.values()) == {'applied'}, result
        if not legacy:
            write(config_path, config_path.read_bytes() + custom)
        assert_config(project)
        repeat(project, home, order)
        print('PASS', label, 'legacy10' if legacy else 'fresh', 'sequential + repeats + approval preservation', flush=True)

    project, home = fixture(label + '-both-prior-receipts')
    for host in order:
        invoke(project, home, host, binary='previous-probe')
    config_path = project / '.codex/config.toml'
    assert config_path.read_bytes() == old_config
    write(config_path, old_config + custom)
    invoke(project, home, order[0])
    content = assert_config(project)
    unchanged = content, config_path.stat().st_mtime_ns
    invoke(project, home, order[1])
    assert (config_path.read_bytes(), config_path.stat().st_mtime_ns) == unchanged, 'receipt adoption rewrote carrier'
    manifests = [json.loads((project / '.haft/host-installations' / (host + '.project.json')).read_text()) for host in order]
    fragments = [[fragment for fragment in manifest['managed_fragments'] if fragment['selector'] == 'mcp_servers.haft'][0] for manifest in manifests]
    assert fragments[0]['digest'] == fragments[1]['digest'], fragments
    repeat(project, home, order)
    print('PASS', label, 'both previous10 manifests reconcile independently; carrier byte/mtime and peer receipts preserved', flush=True)

for replacement in (
    (b'startup_timeout_sec = 10', b'startup_timeout_sec = 45'),
    (b'tool_timeout_sec = 60', b'tool_timeout_sec = 90'),
    (b'tool_timeout_sec = 60', b'tool_timeout_sec = 60\nrequired = true'),
    (b'tool_timeout_sec = 60', b'tool_timeout_sec = 60\napproval_policy = "never"'),
    (b'command = "haft"', b'command = "custom-wrapper"'),
    (b'HAFT_PROJECT_ROOT = "."', b'HAFT_PROJECT_ROOT = "."\nCUSTOM = "retained"'),
):
    for owned in (False, True):
        label = str(len(list(fixtures.iterdir())))
        project, home = fixture('custom-' + label)
        if owned:
            invoke(project, home, 'codex', binary='previous-probe')
            invoke(project, home, 'air', binary='previous-probe')
        config_path = project / '.codex/config.toml'
        config = old_config.replace(*replacement) + custom
        write(config_path, config)
        before = state(project, home)
        for host in ('codex', 'air'):
            error = invoke(project, home, host, succeeds=False)
            assert '.codex/config.toml' in error
            assert state(project, home) == before, 'blocked request modified a fixture'
        print('PASS custom', replacement[1].decode().replace('\n', '; '), 'owned' if owned else 'unowned', 'preserved for both hosts', flush=True)

project, home = fixture('receipt-reconciliation-race')
invoke(project, home, 'codex', binary='previous-probe')
invoke(project, home, 'air', binary='previous-probe')
invoke(project, home, 'codex')
air_manifest = project / '.haft/host-installations/air.project.json'
prior_receipt = air_manifest.read_bytes()
result = invoke(project, home, 'air', mode='race')
assert set(result.values()) == {'precondition_changed'}, result
assert air_manifest.read_bytes() == prior_receipt
assert (project / '.codex/config.toml').read_bytes().endswith(b'\n# Concurrent operator edit.\n')
print('PASS concurrent carrier edit rejects stale-receipt adoption; original receipt retained', flush=True)

assert not list(fixtures.rglob('*.db')), 'probe created a database'
assert environment == {name: os.environ.get(name) for name in environment}
print('PASS no database files; HOME/CODEX_HOME unchanged; fixtures:', fixtures, flush=True)
