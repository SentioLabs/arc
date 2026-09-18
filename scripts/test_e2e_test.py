"""Lifecycle contract tests; real Docker isolation is tested by `isolation`."""
import importlib.util
import pathlib
import json
import os
import subprocess
import tempfile
import sys
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location('harness', pathlib.Path(__file__).with_name('test_e2e.py'))
harness = importlib.util.module_from_spec(spec)
spec.loader.exec_module(harness)


class LifecycleTests(unittest.TestCase):
    def test_invalid_mode_has_no_side_effects(self):
        with patch.object(harness, 'new_run') as create:
            with self.assertRaises(SystemExit):
                harness.main(['invalid'])
            create.assert_not_called()

    def test_failed_start_cleans_only_its_project_and_retains_logs(self):
        with tempfile.TemporaryDirectory() as temp:
            run = harness.new_run(pathlib.Path(temp))
            calls = []

            def compose(state, *args, **kwargs):
                calls.append((state, args))
                if 'up' in args:
                    raise subprocess.CalledProcessError(19, args)
                return subprocess.CompletedProcess(args, 0, stdout='server log')

            with patch.object(harness, 'compose', side_effect=compose):
                with self.assertRaises(subprocess.CalledProcessError):
                    harness.start(run)
            self.assertEqual(calls[-1], (run, ('down', '--volumes', '--remove-orphans', '--rmi', 'local')))
            self.assertIn('server log', (run / 'server.log').read_text())

    def test_test_failure_propagates_and_cleans(self):
        with tempfile.TemporaryDirectory() as temp:
            run = harness.new_run(pathlib.Path(temp))
            with patch.object(harness, 'new_run', return_value=run), \
                 patch.object(harness, 'compose', side_effect=subprocess.CalledProcessError(23, ['run'])), \
                 patch.object(harness, 'stop') as stop:
                with self.assertRaises(subprocess.CalledProcessError) as failure:
                    harness.main(['integration'])
                self.assertEqual(failure.exception.returncode, 23)
                stop.assert_called_once_with(run)

    def test_browser_failure_propagates_and_cleans(self):
        with tempfile.TemporaryDirectory() as temp:
            run = harness.new_run(pathlib.Path(temp))
            with patch.object(harness, 'new_run', return_value=run), \
                 patch.object(harness, 'start'), \
                 patch.object(harness, 'browser_tests', side_effect=subprocess.CalledProcessError(31, ['playwright'])), \
                 patch.object(harness, 'stop') as stop:
                with self.assertRaises(subprocess.CalledProcessError) as failure:
                    harness.main(['playwright'])
                self.assertEqual(failure.exception.returncode, 31)
                stop.assert_called_once_with(run)

    def test_isolation_cleans_second_stack_when_first_log_collection_fails(self):
        with tempfile.TemporaryDirectory() as temp:
            first = harness.new_run(pathlib.Path(temp))
            second = harness.new_run(pathlib.Path(temp))
            (first / 'url').write_text('http://127.0.0.1:32100')
            (second / 'url').write_text('http://127.0.0.1:32101')
            calls = []

            def compose(state, *args, **kwargs):
                calls.append((state, args))
                if state == first and args[0] == 'logs':
                    raise OSError('first stack log collection failed')
                return subprocess.CompletedProcess(args, 0, stdout='second stack log')

            with patch.object(harness, 'new_run', side_effect=[first, second]), \
                 patch.object(harness, 'start') as start, \
                 patch.object(harness, 'request', side_effect=[{'id': 'sentinel'}, [], [{'id': 'sentinel'}]]), \
                 patch.object(harness, 'compose', side_effect=compose):
                with self.assertRaisesRegex(OSError, 'first stack log collection failed'):
                    harness.isolation()
            self.assertEqual(start.call_count, 2)
            down = ('down', '--volumes', '--remove-orphans', '--rmi', 'local')
            self.assertIn((first, down), calls)
            self.assertIn((second, down), calls)
            self.assertEqual((second / 'server.log').read_text(), 'second stack log')

    def test_failed_build_and_browser_diagnostics_survive_owned_cleanup(self):
        for mode, failure_code in [('start', 73), ('playwright', 79)]:
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temp:
                root = pathlib.Path(temp)
                run = harness.new_run(root)
                (run / 'url').write_text('http://127.0.0.1:32100')
                calls = root / 'calls.jsonl'
                program = f'''#!{sys.executable}
import json, sys
with open({str(calls)!r}, 'a') as output:
    output.write(json.dumps(sys.argv[1:]) + '\\n')
if 'up' in sys.argv or 'test' in sys.argv:
    print('INJECTED_COMMAND_FAILURE', file=sys.stderr, flush=True)
    sys.exit(73 if 'up' in sys.argv else 79)
'''
                for name in ['docker', 'bun']:
                    command = root / name
                    command.write_text(program)
                    command.chmod(0o755)
                with patch.dict(os.environ, {'PATH': str(root) + os.pathsep + os.environ['PATH']}), \
                     patch.object(harness, 'new_run', return_value=run):
                    if mode == 'start':
                        with self.assertRaises(subprocess.CalledProcessError) as failure:
                            harness.main([mode])
                    else:
                        with patch.object(harness, 'start'), \
                             self.assertRaises(subprocess.CalledProcessError) as failure:
                            harness.main([mode])
                self.assertEqual(failure.exception.returncode, failure_code)
                commands = [json.loads(line) for line in calls.read_text().splitlines()]
                docker_calls = [args for args in commands if 'compose' in args]
                self.assertIn('down', docker_calls[-1])
                for args in docker_calls:
                    self.assertEqual(args[args.index('--project-name') + 1], harness.identity(run))
                self.assertIn('INJECTED_COMMAND_FAILURE', (run / 'commands.log').read_text())

    def test_captured_discovery_stdout_excludes_retained_stderr(self):
        with tempfile.TemporaryDirectory() as temp:
            run = harness.new_run(pathlib.Path(temp))
            result = harness.run_logged(run, [sys.executable, '-c',
                "import sys; print('127.0.0.1:32100'); print('diagnostic', file=sys.stderr)"],
                capture_output=True)
            self.assertEqual(result.stdout, '127.0.0.1:32100\n')
            self.assertEqual(result.stderr, 'diagnostic\n')
            self.assertIn('diagnostic', (run / 'commands.log').read_text())

    def test_progress_is_displayed_before_command_completes(self):
        with tempfile.TemporaryDirectory() as temp:
            run = harness.new_run(pathlib.Path(temp))
            acknowledged = run / 'progress-acknowledged'
            program = f'''import pathlib, time, sys
print('BUILD_PROGRESS', flush=True)
marker = pathlib.Path({str(acknowledged)!r})
deadline = time.monotonic() + 3
while not marker.exists() and time.monotonic() < deadline:
    time.sleep(0.01)
sys.exit(0 if marker.exists() else 1)
'''
            def progress(line, **kwargs):
                if line == 'BUILD_PROGRESS\n':
                    acknowledged.write_text('displayed while command was running')

            with patch('builtins.print', side_effect=progress):
                harness.run_logged(run, [sys.executable, '-c', program])
            self.assertTrue(acknowledged.exists())

    def test_log_write_failure_does_not_prevent_command_execution(self):
        with tempfile.TemporaryDirectory() as temp:
            run = harness.new_run(pathlib.Path(temp))
            (run / 'commands.log').mkdir()
            executed = run / 'command-executed'
            with self.assertRaises(IsADirectoryError):
                harness.run_logged(run, [sys.executable, '-c',
                    f"from pathlib import Path; Path({str(executed)!r}).touch()"])
            self.assertTrue(executed.exists())

    def test_repeated_stop_preserves_failure_logs(self):
        with tempfile.TemporaryDirectory() as temp:
            run = harness.new_run(pathlib.Path(temp))
            with patch.object(harness, 'compose', return_value=subprocess.CompletedProcess([], 0, stdout='failure log')):
                harness.stop(run)
            with patch.object(harness, 'compose', return_value=subprocess.CompletedProcess([], 0, stdout='')):
                harness.stop(run)
            self.assertEqual((run / 'server.log').read_text(), 'failure log')

    def test_project_identity_is_valid_even_if_tempfile_name_contains_underscores(self):
        with tempfile.TemporaryDirectory() as temp:
            run = pathlib.Path(temp) / 'arc-test-_example'
            run.mkdir()
            with patch.object(harness.tempfile, 'mkdtemp', return_value=str(run)):
                harness.new_run()
            self.assertRegex(harness.identity(run), r'^arc-test-[a-z0-9]+$')

    def test_unique_run_identity_and_private_home(self):
        with tempfile.TemporaryDirectory() as temp:
            first = harness.new_run(pathlib.Path(temp))
            second = harness.new_run(pathlib.Path(temp))
            self.assertNotEqual(first.name, second.name)
            self.assertNotEqual(harness.identity(first), harness.identity(second))
            self.assertEqual(first.stat().st_mode & 0o777, 0o700)

    def test_stop_refuses_unowned_state(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(harness, 'compose') as compose:
            with self.assertRaises(ValueError):
                harness.stop(pathlib.Path(temp))
            compose.assert_not_called()


if __name__ == '__main__':
    unittest.main()
