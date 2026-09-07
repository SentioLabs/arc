import json, os, subprocess, sys
from pathlib import Path
root = Path(sys.argv[1]).resolve()
revision = 'f806cb7bdb47984b2d706d634240c8e29b02f4ae'
source = 'codex-marketplace/plugins/arc/skills/build/SKILL.md'
lines = subprocess.check_output(['git', '-C', str(root), 'show', revision + ':' + source], text=True).splitlines()
results = []
def run(name, fixture, source_line, output_var):
    snippet = lines[source_line - 1] + '\nprintf \'%s\' "$' + output_var + '"\n'
    r = subprocess.run(['bash', '-c', snippet], env={**os.environ, 'TASK_JSON': json.dumps(fixture)}, capture_output=True)
    results.append({'name': name, 'source_line': source_line, 'fixture': fixture, 'exit': r.returncode, 'output_hex': r.stdout.hex(), 'output_repr': repr(r.stdout), 'stderr': r.stderr.decode()})
run('trailing LF preservation', {'description': 'full task\n\n'}, 147, 'TASK_DESCRIPTION')
run('trailing CRLF preservation', {'description': 'full task\r\n\r\n'}, 147, 'TASK_DESCRIPTION')
run('comments projected away', {'description': 'full task', 'comments': [{'id': 1, 'text': 'PROGRESS: retained decision'}]}, 147, 'TASK_DESCRIPTION')
run('unlabeled API issue omitted labels', {'description': 'full task'}, 146, 'BUILD_LABELS')
run('parent dependency only API issue', {'id': 'fixture-parent.1', 'description': 'full task', 'dependencies': [{'issue_id': 'fixture-parent.1', 'depends_on_id': 'fixture-parent', 'type': 'parent-child'}]}, 144, 'TASK_PARENT')
run('populated parent_id selects full object', {'id': 'child', 'parent_id': 'parent'}, 144, 'TASK_PARENT')
print(json.dumps(results, indent=2))
