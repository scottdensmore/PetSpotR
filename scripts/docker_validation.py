"""Read tracked image versions; pin only a temporary validation Dockerfile."""

import argparse
import datetime
import json
import re
from pathlib import Path


FROM = re.compile(
    r"^(?P<prefix>[ \t]*FROM[ \t]+)(?P<image>\S+)"
    r"(?P<suffix>(?:[ \t]+AS[ \t]+[\w.-]+)?[ \t]*)$", re.I | re.M,
)
IMAGE = re.compile(
    r"(?P<repository>golang|docker\.io/library/golang|gcr\.io/distroless/static-debian12)"
    r"(?::(?P<tag>[\w.-]+))?(?:@sha256:(?P<digest>[a-f0-9]{64}))?", re.ASCII,
)
DIGEST = re.compile(r"(?P<repository>.+)@sha256:[a-f0-9]{64}")


def normalized(ref):
    return ref.replace("docker.io/library/golang", "golang", 1)


def source_images(text):
    matches = list(FROM.finditer(text))
    if len(matches) != 2 or len(re.findall(r"^\s*FROM\b", text, re.I | re.M)) != 2:
        raise ValueError("Expected exactly two literal FROM instructions")
    refs = [match["image"] for match in matches]
    for ref, expected in zip(refs, ("golang", "gcr.io/distroless/static-debian12")):
        parsed = IMAGE.fullmatch(ref)
        if (not parsed or normalized(parsed["repository"]) != expected
                or not (parsed["tag"] or parsed["digest"])):
            raise ValueError(f"Unexpected or nonliteral base image: {ref}")
    if matches[0]["suffix"].strip().lower() != "as builder" or matches[1]["suffix"].strip():
        raise ValueError("Expected the builder stage followed by the runtime stage")
    return matches, refs


def pin_images(text, images):
    matches, refs = source_images(text)
    if not isinstance(images, list) or len(images) != 2:
        raise ValueError("Expected inspect records for both pulled images")
    records = []
    for requested, image in zip(refs, images):
        if image.get("Os") != "linux" or image.get("Architecture") != "amd64":
            raise ValueError("Expected linux/amd64 images")
        digests = image.get("RepoDigests", [])
        repository = IMAGE.fullmatch(requested)["repository"]
        if not digests or not all(
            DIGEST.fullmatch(ref)
            and normalized(DIGEST.fullmatch(ref)["repository"]) == normalized(repository)
            for ref in digests
        ):
            raise ValueError("Missing, malformed or unexpected repository digest")
        pulled_refs = (image.get("RepoTags") or []) + digests
        if normalized(requested) not in {normalized(ref) for ref in pulled_refs}:
            raise ValueError(f"Inspect record does not match Dockerfile image: {requested}")
        records.append({
            "requested_tag": requested, "resolved_repo_digests": digests,
            "image_id": image["Id"], "os": image["Os"],
            "architecture": image["Architecture"], "variant": image.get("Variant"),
            "selected_build_ref": digests[0],
        })
    # Replace only the two image tokens. Preserve stage aliases and every other instruction.
    for match, record in reversed(list(zip(matches, records))):
        text = text[:match.start("image")] + record["selected_build_ref"] + text[match.end("image"):]
    return text, records


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    refs = commands.add_parser("refs")
    refs.add_argument("--dockerfile", type=Path, required=True)
    refs.add_argument("--env-file", type=Path)
    pin = commands.add_parser("pin")
    pin.add_argument("--dockerfile", type=Path, required=True)
    pin.add_argument("--images", type=Path, required=True)
    pin.add_argument("--output", type=Path, required=True)
    pin.add_argument("--head", required=True)
    args = parser.parse_args()
    text = args.dockerfile.read_text()
    if args.command == "refs":
        _, images = source_images(text)
        values = dict(zip(("BUILDER_TAG", "RUNTIME_TAG"), images))
        if args.env_file:
            with args.env_file.open("a") as env:
                env.writelines(f"{key}={value}\n" for key, value in values.items())
        print(json.dumps(values))
        return
    if args.output.resolve() == args.dockerfile.resolve():
        parser.error("The tracked Dockerfile must not be overwritten")
    if not re.fullmatch(r"[a-f0-9]{40}", args.head):
        parser.error("Expected the exact approved commit SHA")
    pinned, records = pin_images(text, json.loads(args.images.read_text()))
    with args.output.open("x") as output:
        output.write(pinned)
    print(json.dumps({
        "observed_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "approved_head": args.head, "platform": "linux/amd64", "base_images": records,
    }, indent=2))


if __name__ == "__main__":
    main()
