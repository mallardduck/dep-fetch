# dep-fetch — Renovate Integration

## Overview

The `.bin-deps.yaml` schema is designed so Renovate can bump versions and checksums automatically, following patterns established in [rancher/renovate-config](https://github.com/rancher/renovate-config).

Renovate's checksum bumping relies on the `custom.local` datasource trick — a pattern already in use across the org via `# renovate-local:` markers in Makefiles. The `.bin-deps.yaml` format uses the same marker and the same trick. `dep-fetch` itself is a developer/CI tool for fetching and verifying binaries; Renovate never interacts with it directly.

## How this compares to the current approach

| | `generate-data-sources.sh` (current) | `generate-checksums.sh` (this spec) |
|---|---|---|
| "Should we run?" | Global: any `# renovate-local:` present | Global short-circuit, then per-tool |
| Scope when running | All tools the script knows about | Only tools with markers in this repo |
| API calls for absent tools | Yes — fetches regardless | Zero |
| File types supported | Makefiles | Makefiles, `.bin-deps.yaml`, any file |
| Scales as tool list grows | Cost grows with generator | Cost constant per repo |
| Marker format | `# renovate-local:` | `# renovate-local:` (unchanged) |

The new generator is designed as a **successor**, not a fork. It uses the same `# renovate-local:` marker convention and produces the same `data/*.json` output format — the only changes are per-tool detection and broader file type support.

Repos that do not adopt `.bin-deps.yaml` are unaffected: their Makefile `# renovate-local:` markers are detected and handled identically. No parallel tooling is needed.

---

## `release-checksums` mode

No checksum management is required — checksums are fetched from the release at runtime. Renovate only needs to bump the `version` field.

Add a custom regex manager to `renovate.json` that matches the `# renovate:` comment above each version line:

```json
{
  "customManagers": [
    {
      "customType": "regex",
      "fileMatch": ["\\.bin-deps\\.yaml$"],
      "matchStrings": [
        "# renovate: datasource=(?<datasource>[^\\s]+) depName=(?<depName>[^\\s]+)\\n\\s+version: (?<currentValue>\\S+)"
      ]
    }
  ]
}
```

This is the only Renovate config needed for `release-checksums` tools. Tools using `version: latest` are intentionally excluded from Renovate management — they track latest by design and do not need PRs.

---

## `pinned` mode

Two things must be bumped atomically when a new version is released: the `version` field and every checksum entry. These are handled by two separate custom managers, grouped into a single PR per tool via `groupName`.

### Step 1 — Version bumping

The same regex manager used for `release-checksums` handles `pinned` mode version lines — the comment pattern is identical. No additional config needed here.

### Step 2 — Checksum bumping via `custom.local` datasource

Checksums are bumped using the same `# renovate-local:` marker pattern already established for Makefile-based deps. Because the YAML key already encodes the platform (`linux/amd64`, `darwin/arm64`, etc.), the marker only needs the tool name and version — the regex manager extracts `os` and `arch` from the key and composes the full `depName` via `depNameTemplate`.

Example `.bin-deps.yaml` entries:

```yaml
tools:
  - name: golangci-lint
    mode: pinned
    # renovate: datasource=github-releases depName=golangci/golangci-lint
    version: v1.57.2
    checksums:
      linux/amd64:  "abc123..."  # renovate-local: golangci-lint=v1.57.2
      linux/arm64:  "def456..."  # renovate-local: golangci-lint=v1.57.2
      darwin/amd64: "ghi789..."  # renovate-local: golangci-lint=v1.57.2
      darwin/arm64: "jkl012..."  # renovate-local: golangci-lint=v1.57.2
```

The Renovate config for checksum bumping:

```json
{
  "customDatasources": {
    "local": {
      "defaultRegistryUrlTemplate": "file://data/{{packageName}}.json",
      "format": "json"
    }
  },
  "customManagers": [
    {
      "customType": "regex",
      "fileMatch": ["\\.bin-deps\\.yaml$"],
      "matchStrings": [
        "(?<os>linux|darwin)/(?<arch>amd64|arm64):\\s+\"(?<currentDigest>[a-f0-9]{64})\"\\s+# renovate-local: (?<depName>[^=\\n]+)=(?<currentValue>[^\\n]+)"
      ],
      "datasourceTemplate": "custom.local",
      "depNameTemplate": "{{depName}}-{{os}}-{{arch}}",
      "versioningTemplate": "loose"
    }
  ],
  "packageRules": [
    {
      "matchDatasources": ["github-releases", "custom.local"],
      "matchPackagePatterns": ["^golangci-lint"],
      "groupName": "golangci-lint"
    }
  ]
}
```

The `groupName` rule ensures the version bump and all platform checksum bumps land in a single PR. One `packageRule` entry is needed per `pinned` tool.

### Step 3 — Data files

The `custom.local` datasource reads from `data/{name}-{os}-{arch}.json` files committed to the repo. These files are regenerated whenever new releases of `pinned` tools are available — typically run as part of the same workflow that triggers the Renovate PR.

Format:

```json
{
  "releases": [
    { "version": "v1.58.0", "digest": "newsha256hex..." },
    { "version": "v1.57.2", "digest": "abc123...64char-hex..." }
  ]
}
```

> **Note**: Data files are a build-time artifact, not a security boundary. Security for `pinned` mode comes from the checksum values in `.bin-deps.yaml` itself — those are what `dep-fetch` verifies at download time. Data files only tell Renovate what checksums exist for new versions.

---

## Data Generator (`hack/generate-checksums.sh`)

### Problem: global detection does not scale

[rancher/renovate-config's `generate-data-sources.sh`](https://github.com/rancher/renovate-config/blob/main/hack/generate-data-sources.sh) uses a single global check: if any `# renovate-local:` marker exists anywhere in the repo, run the entire script and fetch checksums for every tool it knows about.

This works for a small, fixed tool set. As the shared generator accumulates more tools, every repo that adopts it pays the cost of API calls and checksum fetches for tools it does not use.

### Solution: per-tool detection across all file types

The generator checks, per tool, whether the repo references it — regardless of whether the marker appears in a Makefile, a `.bin-deps.yaml`, or any other file. Two marker formats are supported:

- **`.bin-deps.yaml` format**: `# renovate-local: {name}={version}` — the `=` delimiter ensures `helm=` cannot match `helm-docs=`.
- **Makefile format**: `# renovate-local: {name}-{arch}={version}` — the platform suffix prevents prefix false positives.

Detection grep for a tool named `{name}`:

```bash
grep -rqE "# renovate-local: ${name}(=|-(amd64|arm64|s390x)=)" .
```

### Generator structure

```bash
#!/usr/bin/env bash
# hack/generate-checksums.sh

set -euo pipefail

# Global short-circuit: nothing to do if no renovate-local checksum markers exist at all.
if ! grep -rqE "# renovate-local: " .; then
    echo "No renovate-local markers found. Skipping data generation."
    exit 0
fi

# Registry of known tools: name -> GitHub owner/repo.
# Adding a new tool here is the only change needed to support it across all repos.
declare -A TOOLS=(
    ["golangci-lint"]="golangci/golangci-lint"
    ["helm"]="helm/helm"
    ["kubectl"]="kubernetes/kubernetes"
    ["kustomize"]="kubernetes-sigs/kustomize"
    # ... extend as new pinned tools are added
)

PLATFORMS=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64")
RELEASE_COUNT=5   # how many recent releases to include per data file

mkdir -p data

for tool_name in "${!TOOLS[@]}"; do
    source_repo="${TOOLS[$tool_name]}"

    # Per-tool detection: matches both .bin-deps.yaml format (name=version) and
    # Makefile format (name-arch=version).
    if ! grep -rqE "# renovate-local: ${tool_name}(=|-(amd64|arm64|s390x)=)" .; then
        echo "Skipping ${tool_name}: no markers found in this repo"
        continue
    fi

    echo "Generating data files for ${tool_name} (${source_repo})..."

    versions=$(gh release list --repo "${source_repo}" \
        --limit "${RELEASE_COUNT}" --json tagName --jq '.[].tagName')

    for platform in "${PLATFORMS[@]}"; do
        os="${platform%/*}"
        arch="${platform#*/}"
        outfile="data/${tool_name}-${os}-${arch}.json"
        releases_json="[]"

        while IFS= read -r version; do
            # fetch_checksum is defined in the existing generate-data-sources.sh
            # and handles the per-tool logic for locating the correct release asset.
            digest=$(fetch_checksum "${source_repo}" "${version}" "${os}" "${arch}" || true)

            if [[ -n "${digest}" ]]; then
                releases_json=$(echo "${releases_json}" | \
                    jq --arg v "${version}" --arg d "${digest}" \
                    '. + [{"version": $v, "digest": $d}]')
            fi
        done <<< "${versions}"

        echo "${releases_json}" | jq '{"releases": .}' > "${outfile}"
        echo "  wrote ${outfile}"
    done
done
```

The `fetch_checksum` helper is inherited from the existing `generate-data-sources.sh` in rancher/renovate-config. The new generator evolves the detection and scoping logic around it, not the per-tool checksum fetching logic itself.

---

## Updates needed in rancher/renovate-config

Adopting `dep-fetch` across the org requires the following changes to the shared [rancher/renovate-config](https://github.com/rancher/renovate-config) repo:

### 1. Add the `.bin-deps.yaml` regex manager to shared presets

The version-bumping regex manager (for both modes) should be published as a reusable preset so repos don't each define it from scratch:

```json
// renovate-config/presets/dep-fetch.json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "customManagers": [
    {
      "customType": "regex",
      "fileMatch": ["\\.bin-deps\\.yaml$"],
      "matchStrings": [
        "# renovate: datasource=(?<datasource>[^\\s]+) depName=(?<depName>[^\\s]+)\\n\\s+version: (?<currentValue>\\S+)"
      ]
    }
  ]
}
```

Repos opt in via:

```json
{ "extends": ["github>rancher/renovate-config:dep-fetch"] }
```

### 2. Extend the `custom.local` datasource preset

The existing `custom.local` datasource config and checksum regex manager should be extended (or a new preset added) to also match checksum lines in `.bin-deps.yaml`. Per-tool `packageRules` (for `groupName`) remain repo-local since they are tool-specific.

### 3. Replace `generate-data-sources.sh` with `generate-checksums.sh`

The new generator is a drop-in for existing repos (same marker format, same output). It should replace the original in rancher/renovate-config once ready.

### 4. Deprecation path for `generate-data-sources.sh`

Once `generate-checksums.sh` is in place, the original script can be removed after all consumer repos have been updated to reference the new one. Existing repos require no other changes since the marker format is identical.
