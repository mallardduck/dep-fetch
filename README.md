# dep-fetch

Fetch versioned binary dependencies from GitHub Releases with checksum verification — replace ad-hoc curl scripts with a single declarative config.

## GitHub Actions (primary usage)

Add the reusable action to any workflow. After it runs, all declared tools are available on `PATH`.

```yaml
steps:
  - uses: actions/checkout@v4

  - uses: rancher/dep-fetch/actions/sync-deps@v0.1.0
    with:
      version: v0.1.0

  - name: Run golangci-lint
    run: golangci-lint run ./...
```

Pin `version` to a specific release for production workflows. Omit it to always pull the latest.

## Developer workstations

Download the binary for your platform from [Releases](https://github.com/mallardduck/dep-fetch/releases), or:

```sh
go install github.com/mallardduck/dep-fetch@latest
```

Then run the same commands locally:

```sh
dep-fetch sync     # fetch and verify all tools into ./bin
dep-fetch verify   # verify checksums without re-fetching
dep-fetch list     # show installed vs declared versions
```
