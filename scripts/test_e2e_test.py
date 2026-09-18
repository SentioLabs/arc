"""Lifecycle contract tests; real Docker isolation is tested by `isolation`."""
import importlib.util
import pathlib
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
