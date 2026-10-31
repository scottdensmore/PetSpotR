import copy
import json
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from docker_validation import pin_images, source_images

ROOT = Path(__file__).resolve().parents[1]
RUNTIME = "gcr.io/distroless/static-debian12:nonroot"


def dockerfile(version="1.27.3"):
    return (f"FROM golang:{version}-alpine AS builder\n"
            "RUN echo build\nARG SERVICE_NAME\n"
            f"FROM {RUNTIME}\nCOPY --from=builder /server /server\n"
            'USER nonroot:nonroot\nENTRYPOINT ["/server"]\n')


def inspected(version="1.27.3"):
    return [
        {"Os": "linux", "Architecture": "amd64", "Id": "sha256:" + "1" * 64,
         "RepoTags": [f"golang:{version}-alpine"], "RepoDigests": ["golang@sha256:" + "a" * 64]},
        {"Os": "linux", "Architecture": "amd64", "Id": "sha256:" + "2" * 64,
         "RepoTags": [RUNTIME],
         "RepoDigests": ["gcr.io/distroless/static-debian12@sha256:" + "b" * 64]},
    ]


class RepositoryImageSourceTests(unittest.TestCase):
    def test_dockerfile_bases_remain_literal_for_dependency_monitoring(self):
        refs = re.findall(r"^FROM\s+(\S+)", (ROOT / "Dockerfile").read_text(), re.MULTILINE)
        self.assertEqual(len(refs), 2)
        self.assertTrue(all("$" not in ref for ref in refs), refs)

    def test_workflow_does_not_maintain_a_second_image_version(self):
        workflow = (ROOT / ".github/workflows/ci.yml").read_text()
        self.assertNotRegex(workflow, r"(?m)^\s+(?:BUILDER_TAG|RUNTIME_TAG):\s+\S+")


class ImageSelectionTests(unittest.TestCase):
    def test_dockerfile_version_update_selects_the_new_image(self):
        self.assertEqual(source_images(dockerfile())[1][0], "golang:1.27.3-alpine")
        self.assertEqual(source_images(dockerfile("1.27.4"))[1][0], "golang:1.27.4-alpine")
        _, records = pin_images(dockerfile("1.27.4"), inspected("1.27.4"))
        self.assertEqual(records[0]["requested_tag"], "golang:1.27.4-alpine")

    def test_old_inspect_record_cannot_validate_a_new_dockerfile_version(self):
        with self.assertRaisesRegex(ValueError, "does not match Dockerfile"):
            pin_images(dockerfile("1.27.4"), inspected("1.27.3"))

    def test_digest_substitution_preserves_every_other_instruction(self):
        original = dockerfile()
        result, records = pin_images(original, inspected())
        expected = original.replace("golang:1.27.3-alpine", records[0]["selected_build_ref"])
        expected = expected.replace(RUNTIME, records[1]["selected_build_ref"])
        self.assertEqual(result, expected)
        self.assertIn(" AS builder\n", result)
        self.assertEqual(original, dockerfile())

    def test_unexpected_image_sources_or_nonliteral_stages_fail_closed(self):
        for source in [
            dockerfile().replace("golang:1.27.3-alpine", "${GO_BUILDER_IMAGE}"),
            dockerfile().replace("golang:1.27.3-alpine", "other/golang:1.27.3-alpine"),
            dockerfile() + "FROM alpine:3.24\n",
        ]:
            with self.subTest(source=source), self.assertRaises(ValueError):
                source_images(source)

    def test_wrong_platform_missing_digest_and_wrong_registry_are_rejected(self):
        for field, value in [("Os", "windows"), ("Architecture", "arm64"),
                             ("RepoDigests", []),
                             ("RepoDigests", ["other/golang@sha256:" + "a" * 64]),
                             ("RepoDigests", ["golang@sha256:not-a-digest"])]:
            images = copy.deepcopy(inspected())
            images[0][field] = value
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                pin_images(dockerfile(), images)

    def test_cli_exports_current_refs_and_writes_only_a_temporary_file(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, env, images, output = [root / name for name in
                                           ("Dockerfile", "env", "images.json", "Dockerfile.validation")]
            source.write_text(dockerfile("1.27.4"))
            images.write_text(json.dumps(inspected("1.27.4")))
            script = str(ROOT / "scripts/docker_validation.py")
            subprocess.run([sys.executable, script, "refs", "--dockerfile", str(source),
                            "--env-file", str(env)], capture_output=True, text=True, check=True)
            self.assertIn("BUILDER_TAG=golang:1.27.4-alpine\n", env.read_text())
            command = [sys.executable, script, "pin", "--dockerfile", str(source),
                       "--images", str(images), "--output", str(output), "--head", "f" * 40]
            result = subprocess.run(command, capture_output=True, text=True, check=True)
            self.assertEqual(json.loads(result.stdout)["base_images"][0]["requested_tag"],
                             "golang:1.27.4-alpine")
            self.assertEqual(source.read_text(), dockerfile("1.27.4"))
            self.assertIn("FROM golang@sha256:", output.read_text())
            # Never overwrite a tracked file or an already existing validation artifact.
            self.assertNotEqual(subprocess.run(command, capture_output=True).returncode, 0)
            command[command.index("--output") + 1] = str(source)
            self.assertNotEqual(subprocess.run(command, capture_output=True).returncode, 0)
            self.assertEqual(source.read_text(), dockerfile("1.27.4"))


if __name__ == "__main__":
    unittest.main()
