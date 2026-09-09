"""Isolated bootstrap/legacy installer checks; no real Arc or daemon is invoked."""
import hashlib
import io
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("install.sh").resolve()
FAKE_ARC = """#!/bin/sh
case "$2" in
 status) printf '{"running":%s}\\n' "$(cat "$FIXTURE/state")" ;;
 stop) echo stop >> "$FIXTURE/events"; echo false > "$FIXTURE/state" ;;
 start) echo start >> "$FIXTURE/events"; echo true > "$FIXTURE/state" ;;
esac
"""


class InstallTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        (self.root / "home").mkdir()
        (self.root / "tmp").mkdir()
        (self.root / "state").write_text("false\n")
        (self.root / "events").touch()
        self.env = dict(os.environ, PATH=f"{self.bin}:/usr/bin:/bin",
                        HOME=str(self.root / "home"), FIXTURE=str(self.root),
                        TMPDIR=str(self.root / "tmp"), FORCE="false", TAG="")
        self.write_command("arc", FAKE_ARC)
        self.write_command("uname", '#!/bin/sh\ncase "$1" in -s) echo Linux;; -m) echo x86_64;; esac\n')
        self.write_command("curl", """#!/bin/sh
url="$2"
out="$4"
echo "$url" >> "$FIXTURE/downloads"
case "$url" in
 */latest) echo '{"tag_name":"v1.1.0"}' > "$out" ;;
 */checksums.txt) cp "$FIXTURE/checksums.txt" "$out" ;;
 *.tar.gz) cp "$FIXTURE/archive.tar.gz" "$out" ;;
 *) exit 22 ;;
esac
""")
        self.payload = (FAKE_ARC + "# updated\n").encode()
        with tarfile.open(self.root / "archive.tar.gz", "w:gz") as archive:
            info = tarfile.TarInfo("arc")
            info.size = len(self.payload)
            info.mode = 0o755
            archive.addfile(info, io.BytesIO(self.payload))
        digest = hashlib.sha256((self.root / "archive.tar.gz").read_bytes()).hexdigest()
        (self.root / "checksums.txt").write_text(f"{digest}  arc_1.1.0_linux_amd64.tar.gz\n")

    def write_command(self, name, body):
        path = self.bin / name
        path.write_text(body)
        path.chmod(0o755)

    def run_install(self, *args, success=True):
        result = subprocess.run(["/bin/bash", str(SCRIPT), *args], env=self.env,
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode == 0, success, result.stdout + result.stderr)
        self.assertEqual(list((self.root / "tmp").iterdir()), [])
        self.assertEqual(list(self.bin.glob(".arc-install.*")), [])
        return result

    def test_existing_install_guidance(self):
        result = self.run_install()
        self.assertIn("arc self update", result.stdout)
        self.assertFalse((self.root / "downloads").exists())
        self.assertEqual((self.bin / "arc").read_text(), FAKE_ARC)

    def test_legacy_update_restarts_running_server(self):
        (self.root / "state").write_text("true\n")
        self.run_install("--force", "--tag=v1.1.0")
        self.assertEqual((self.bin / "arc").read_bytes(), self.payload)
        self.assertEqual((self.root / "events").read_text(), "stop\nstart\n")

    def test_legacy_update_leaves_stopped_server_stopped(self):
        self.env.update(FORCE="true", TAG="v1.1.0")
        self.run_install()
        self.assertEqual((self.bin / "arc").read_bytes(), self.payload)
        self.assertEqual((self.root / "events").read_text(), "")

    def test_checksum_failure(self):
        (self.root / "checksums.txt").write_text("0" * 64 + "  arc_1.1.0_linux_amd64.tar.gz\n")
        self.run_install("--tag", "v1.1.0", success=False)
        self.assertEqual((self.bin / "arc").read_text(), FAKE_ARC)
        self.assertEqual((self.root / "events").read_text(), "")

    def test_missing_asset(self):
        (self.root / "archive.tar.gz").unlink()
        self.run_install("--force", success=False)
        self.assertEqual((self.bin / "arc").read_text(), FAKE_ARC)
        self.assertEqual((self.root / "events").read_text(), "")

    def test_failed_replace_recovers_server(self):
        (self.root / "state").write_text("true\n")
        self.write_command("mv", "#!/bin/sh\nexit 1\n")
        self.run_install("--force", success=False)
        self.assertEqual((self.bin / "arc").read_text(), FAKE_ARC)
        self.assertEqual((self.root / "events").read_text(), "stop\nstart\n")

    def test_fresh_install(self):
        if os.access("/usr/local/bin", os.W_OK):
            self.skipTest("Fresh-install default is system-writable; do not touch it")
        (self.bin / "arc").unlink()
        self.assertIsNone(shutil.which("arc", path=self.env["PATH"]))
        self.run_install()
        self.assertEqual((self.root / "home/.local/bin/arc").read_bytes(), self.payload)
        self.assertEqual((self.root / "events").read_text(), "")


if __name__ == "__main__":
    unittest.main()
