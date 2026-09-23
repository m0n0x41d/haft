#!/usr/bin/env python3
"""Run controlled fixture probes. These are not agent comparison attempts."""
import argparse
import datetime
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

p = argparse.ArgumentParser()
p.add_argument('--output', type=Path, required=True)
a = p.parse_args()
a.output.mkdir(parents=True, exist_ok=False)
root = Path(__file__).resolve().parents[2]
sha = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
scenarios = [
    ('property_pass', 'TestCancelPreservesTotal', None, 0),
    ('scenario_pass', 'TestCancelStates', None, 0),
    ('assertion_failure', 'TestCancelPreservesTotal', 'property', 1),
    ('dependency_failure', 'TestCancelPreservesTotal', 'dependency', 1),
    ('irrelevant_pass', 'TestCurrency', 'property', 0),
    ('zero_tests', 'NoSuchTest', None, 0),
    ('skipped', 'TestExternalSettlement', None, 0),
    ('environment_failure', 'TestCancelPreservesTotal', 'compile', 1),
]
results = []
# Homes, fixtures and outputs stay inside this explicitly selected probe directory.
for label, selector, mutation, expected in scenarios:
    case = a.output / label
    shutil.copytree(root / 'internal/core/testdata/order', case)
    if mutation == 'property':
        f = case / 'order.go'
        f.write_text(f.read_text().replace('o.Status = "cancelled"', 'o.Total++\n\to.Status = "cancelled"'))
    if mutation == 'dependency':
        f = case / 'policy.go'
        f.write_text(f.read_text().replace(' || status == "paid"', ''))
    if mutation == 'compile':
        (case / 'broken.go').write_text('package orders\nvar broken = missingDependency\n')
    env = os.environ.copy()
    child_home = a.output.resolve() / 'home'
    child_home.mkdir(exist_ok=True)
    env.update(HOME=str(child_home), XDG_CACHE_HOME=str(child_home / 'cache'),
               XDG_CONFIG_HOME=str(child_home / 'config'), XDG_DATA_HOME=str(child_home / 'data'),
               GOCACHE=str(a.output.resolve() / 'go-build'), GOWORK='off')
    cmd = ['go', 'test', '-count=1', '-json', '-run', '^' + selector + '$', './...']
    run = subprocess.run(cmd, cwd=case, env=env, capture_output=True)
    (case / 'stdout.jsonl').write_bytes(run.stdout)
    (case / 'stderr.txt').write_bytes(run.stderr)
    events = []
    for line in run.stdout.splitlines():
        try:
            events.append(json.loads(line))
        except ValueError:
            pass
    terminal = [{'test': x.get('Test'), 'action': x['Action']} for x in events
                if x.get('Test') and x.get('Action') in ('pass', 'fail', 'skip')]
    results.append(dict(case=label, selector=selector, mutation=mutation, command=cmd,
                        exit_code=run.returncode, expected_exit=expected,
                        tests=terminal, candidate_sha=sha,
                        expected_scope='direct domain cancellation' if label != 'irrelevant_pass'
                        else 'currency only; does not support cancellation',
                        expected_outcome_observed=run.returncode == expected))
report = dict(format='haft.fixture-probe/1', candidate_sha=sha,
              created_at=datetime.datetime.now(datetime.timezone.utc).isoformat(),
              kind='controlled_fixture_edits_not_agent_attempts', cases=results,
              limits=['exit and test events only; v10 adapter not invoked by this script',
                      'fixture pass is not proof for every input or all claims'])
(a.output / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps({'output': str(a.output), 'cases': len(results),
                  'expected_exits': all(r['expected_outcome_observed'] for r in results)}))
raise SystemExit(not all(r['expected_outcome_observed'] for r in results))
