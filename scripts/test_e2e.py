#!/usr/bin/env python3
"""Own each test stack explicitly; never discover or reuse a running Arc server."""
import argparse
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import uuid
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parent.parent
COMPOSE = ROOT / 'docker-compose.test.yml'


def new_run(parent=None):
    state = Path(tempfile.mkdtemp(prefix='arc-test-', dir=parent))
    (state / 'project').write_text(f'arc-test-{uuid.uuid4().hex}')
    (state / 'owner').write_text('arc-isolated-tests-v1')
    print(f'Test state: {state}', flush=True)
    return state


def identity(state):
    if (state / 'owner').read_text() != 'arc-isolated-tests-v1':
        raise ValueError('Not an Arc test state directory')
    project = (state / 'project').read_text()
    if not re.fullmatch(r'arc-test-[a-z0-9]+', project):
        raise ValueError('Invalid test project identity')
    return project


def run_logged(state, command, capture_output=False, **kwargs):
    output = []
    try:
        if capture_output:
            # Discovery needs clean stdout, separate from Docker's diagnostics.
            result = subprocess.run(command, capture_output=True, text=True, **kwargs)
            output.extend([result.stdout, result.stderr])
        else:
            with subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                  text=True, **kwargs) as process:
                try:
                    for line in process.stdout:
                        output.append(line)
                        print(line, end='', flush=True)
                    result = subprocess.CompletedProcess(command, process.wait())
                except BaseException:
                    process.kill()
                    process.wait()
                    raise
    finally:
        # Run the command before writing logs: a log write failure must never
        # prevent a teardown command from attempting to remove owned resources.
        with (state / 'commands.log').open('a') as log:
            log.write(json.dumps(command) + '\n')
            log.writelines(output)
    result.check_returncode()
    return result


def compose(state, *args, **kwargs):
    project = identity(state)
    return run_logged(state,
        ['docker', 'compose', '-f', str(COMPOSE), '--project-name', project,
         '--profile', 'integration', '--profile', 'playwright', *args], **kwargs)


def stop(state):
    try:
        identity(state)
    except (OSError, ValueError) as error:
        raise ValueError(f'Refusing to stop unowned state: {state}') from error
    # Logs and browser artifacts survive teardown, including failed starts.
    try:
        logs = compose(state, 'logs', '--no-color', capture_output=True)
        with (state / 'server.log').open('a') as output:
            output.write(logs.stdout)
    finally:
        compose(state, 'down', '--volumes', '--remove-orphans', '--rmi', 'local')


def start(state):
    try:
        compose(state, 'up', '-d', '--build', '--wait', 'arc-test-server')
        address = compose(state, 'port', 'arc-test-server', '7432', capture_output=True).stdout.strip()
        if not re.fullmatch(r'127\.0\.0\.1:\d+', address):
            raise ValueError(f'Expected a dynamically allocated loopback port, got {address!r}')
        base = f'http://{address}'
        (state / 'url').write_text(base)
        with urlopen(f'{base}/health', timeout=10) as response:
            if response.status != 200:
                raise RuntimeError('Test server failed health check')
        print(f'Arc sandbox: {base}\nStop: scripts/test-e2e.sh stop {state}', flush=True)
    except BaseException:
        stop(state)
        raise


def browser_tests(state):
    base = (state / 'url').read_text()
    env = dict(os.environ, ARC_TEST_BASE_URL=base,
               PLAYWRIGHT_HTML_OUTPUT_DIR=str(state / 'playwright-report'))
    run_logged(state, ['bun', 'x', 'playwright', 'install', 'chromium'], cwd=ROOT / 'web', env=env)
    run_logged(state, ['bun', 'x', 'playwright', 'test', '--config', 'playwright.e2e.config.ts',
                    '--output', str(state / 'test-results')], cwd=ROOT / 'web', env=env)


def request(base, path, body=None):
    data = None if body is None else json.dumps(body).encode()
    with urlopen(Request(base + path, data=data, headers={'Content-Type': 'application/json'}), timeout=10) as response:
        return json.load(response)


def isolation():
    first, second = new_run(), new_run()
    try:
        start(first)
        start(second)
        a, b = (first / 'url').read_text(), (second / 'url').read_text()
        assert identity(first) != identity(second)
        assert a != b
        project = request(a, '/api/v1/projects', {'name': 'Isolation sentinel', 'path': '/tmp/sentinel', 'prefix': 'ISO'})
        projects = request(b, '/api/v1/projects')
        assert projects == [], projects
        assert any(p['id'] == project['id'] for p in request(a, '/api/v1/projects'))
        stop(first)
        assert request(b, '/health')['status'] == 'healthy'
        assert request(b, '/api/v1/projects') == []
        print(f'PASS: distinct projects and ports, private databases, selective teardown: {a}, {b}')
    finally:
        try:
            stop(first)
        finally:
            stop(second)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', nargs='?', default='all', choices=['integration', 'playwright', 'all', 'start', 'stop', 'isolation'])
    parser.add_argument('state', nargs='?', type=Path)
    args = parser.parse_args(argv)
    if (args.mode == 'stop') != (args.state is not None):
        parser.error('Only stop requires a state directory')
    if args.mode == 'stop':
        stop(args.state)
        return
    if args.mode == 'isolation':
        isolation()
        return
    state = new_run()
    if args.mode == 'start':
        start(state)
        return
    try:
        if args.mode in ('integration', 'all'):
            compose(state, 'up', '--build', '--abort-on-container-exit', '--exit-code-from', 'integration-tests', 'integration-tests')
        if args.mode in ('playwright', 'all'):
            start(state)
            browser_tests(state)
    finally:
        stop(state)


if __name__ == '__main__':
    def interrupted(signum, _frame):
        raise SystemExit(128 + signum)

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    try:
        main()
    except subprocess.CalledProcessError as error:
        sys.exit(error.returncode)
