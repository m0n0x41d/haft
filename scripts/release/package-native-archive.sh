#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 6 ]; then
  echo "usage: package-native-archive.sh VERSION CANDIDATE_SHA GOOS GOARCH EXECUTABLE OUTPUT_DIR" >&2
  exit 64
fi

version="$1"
candidate_sha="$2"
target_os="$3"
target_arch="$4"
executable="$5"
output_dir="$6"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] ||
   [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "native archive requires exact version and candidate SHA" >&2
  exit 1
fi
host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
if [ "$host_os" != "$target_os" ] || [ "$host_arch" != "$target_arch" ]; then
  echo "native archive target $target_os-$target_arch differs from host $host_os-$host_arch" >&2
  exit 1
fi
case "$target_os-$target_arch" in
  linux-amd64|linux-arm64|darwin-arm64) ;;
  *)
    echo "unsupported native release target: $target_os-$target_arch" >&2
    exit 1
    ;;
esac

if [ -L "$executable" ]; then
  echo "native archive executable must not be a symlink: $executable" >&2
  exit 1
fi
physical_executable=$(python3 - "$executable" <<'PY'
import os
import sys
print(os.path.realpath(sys.argv[1]))
PY
)
if [ ! -f "$physical_executable" ] || [ ! -x "$physical_executable" ]; then
  echo "native archive executable must resolve to an executable regular file" >&2
  exit 1
fi

mkdir -p "$output_dir"
archive_name="haft-${target_os}-${target_arch}.tar.gz"
archive_path="$output_dir/$archive_name"
receipt_path="$output_dir/haft-${target_os}-${target_arch}.version-receipt.json"
if [ -e "$archive_path" ] || [ -e "$receipt_path" ]; then
  echo "native archive output already exists for $target_os-$target_arch" >&2
  exit 1
fi

temporary=$(mktemp -d "${TMPDIR:-/tmp}/haft-native-package.XXXXXX")
cleanup() {
  rm -rf -- "$temporary"
}
trap cleanup EXIT
version_output="$temporary/version.txt"
"$physical_executable" version > "$version_output"
test "$(wc -l < "$version_output" | tr -d ' ')" = 4
grep -Fx "haft $version" "$version_output" >/dev/null
grep -Fx "  commit:  $candidate_sha" "$version_output" >/dev/null
grep -Eq '^  built:   .+$' "$version_output"
grep -Eq '^  source:  .+$' "$version_output"
if grep -Fq '  modified: true' "$version_output"; then
  echo "native archive executable reports modified source" >&2
  exit 1
fi

python3 - "$physical_executable" "$archive_path" <<'PY'
import gzip
import os
import sys
import tarfile

source, destination = sys.argv[1:]
content = open(source, "rb").read()
with open(destination, "xb") as raw:
    with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode="w", format=tarfile.USTAR_FORMAT) as archive:
            entry = tarfile.TarInfo("haft")
            entry.size = len(content)
            entry.mode = 0o755
            entry.mtime = 0
            entry.uid = 0
            entry.gid = 0
            entry.uname = ""
            entry.gname = ""
            import io
            archive.addfile(entry, io.BytesIO(content))
PY

digest_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print "sha256:" $1}'
  else
    shasum -a 256 "$1" | awk '{print "sha256:" $1}'
  fi
}
archive_digest=$(digest_file "$archive_path")
executable_digest=$(digest_file "$physical_executable")
python3 - "$archive_path" "$physical_executable" <<'PY'
import gzip
import sys
import tarfile

archive_path, executable_path = sys.argv[1:]
with gzip.open(archive_path, "rb") as compressed:
    with tarfile.open(fileobj=compressed, mode="r:") as archive:
        members = archive.getmembers()
        if len(members) != 1 or members[0].name != "haft":
            raise SystemExit("native archive must contain exactly one haft member")
        member = members[0]
        if not member.isfile() or member.mode != 0o755 or member.mtime != 0:
            raise SystemExit("native archive member metadata is invalid")
        archived = archive.extractfile(member)
        if archived is None:
            raise SystemExit("native archive member is unreadable")
        with open(executable_path, "rb") as expected:
            if archived.read() != expected.read():
                raise SystemExit("native archive member differs from the executable")
PY
version_output_digest=$(digest_file "$version_output")
version_output_base64=$(base64 < "$version_output" | tr -d '\n')

RECEIPT_SCHEMA='haft.p14.native-version-receipt/v1' \
ARCHIVE_NAME="$archive_name" \
ARCHIVE_DIGEST="$archive_digest" \
CANDIDATE_SHA="$candidate_sha" \
VERSION="$version" \
TARGET_OS="$target_os" \
TARGET_ARCH="$target_arch" \
EXECUTABLE_DIGEST="$executable_digest" \
VERSION_OUTPUT_BASE64="$version_output_base64" \
VERSION_OUTPUT_DIGEST="$version_output_digest" \
python3 - "$receipt_path" <<'PY'
import json
import os
import sys

receipt = {
    "schema": os.environ["RECEIPT_SCHEMA"],
    "archive_name": os.environ["ARCHIVE_NAME"],
    "archive_digest": os.environ["ARCHIVE_DIGEST"],
    "candidate_sha": os.environ["CANDIDATE_SHA"],
    "version": os.environ["VERSION"],
    "goos": os.environ["TARGET_OS"],
    "goarch": os.environ["TARGET_ARCH"],
    "executable_digest": os.environ["EXECUTABLE_DIGEST"],
    "version_output_base64": os.environ["VERSION_OUTPUT_BASE64"],
    "version_output_digest": os.environ["VERSION_OUTPUT_DIGEST"],
}
with open(sys.argv[1], "x", encoding="utf-8", newline="\n") as destination:
    json.dump(receipt, destination, indent=2)
    destination.write("\n")
PY

printf 'native archive packaged: sha=%s version=%s target=%s-%s archive=%s receipt=%s\n' \
  "$candidate_sha" "$version" "$target_os" "$target_arch" "$archive_path" "$receipt_path"
