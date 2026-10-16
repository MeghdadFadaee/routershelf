"""Exercise the repository checker against disposable Git working trees."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

CHECKER = Path(__file__).resolve().parents[1] / "check_repository.py"


class RepositoryChecks(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="routershelf-source-check-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        subprocess.run(["git", "init", "--quiet", str(self.root)], check=True)
        (self.root / ".gitignore").write_text(".env\noutputs/\n")
        (self.root / "README.md").write_text("[Guide](guide.md)\n")
        (self.root / "guide.md").write_text("Public guide\n")
        (self.root / "project.json").write_text(json.dumps({
            "services": {"web": {"entrypoint": "README.md",
                                  "configuration": "guide.md"}}}))

    def check(self):
        return subprocess.run([sys.executable, "-B", str(CHECKER)],
                              cwd=self.root, capture_output=True, text=True)

    def test_ignored_private_data_is_excluded(self):
        (self.root / ".env").write_text("do not read or expose this private value")
        (self.root / "outputs").mkdir()
        (self.root / "outputs" / "private.md").write_text("[Broken](absent)")
        result = self.check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("private value", result.stdout + result.stderr)

    def test_broken_links_and_private_source_are_rejected(self):
        (self.root / "README.md").write_text("[Missing](absent.md)\n")
        (self.root / "account.key").write_text("must never appear in diagnostics")
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("broken local link absent.md", result.stderr)
        self.assertIn("account.key: private/generated", result.stderr)
        self.assertNotIn("must never appear", result.stdout + result.stderr)

    def test_symlink_cannot_open_ignored_private_data(self):
        (self.root / ".env").write_text("private-data-sentinel")
        (self.root / "leak.md").symlink_to(".env")
        result = self.check()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("leak.md: symlinks", result.stderr)
        self.assertNotIn("private-data-sentinel", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
