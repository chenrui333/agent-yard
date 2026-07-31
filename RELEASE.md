# Release Process

This repository publishes immutable versioned releases with GoReleaser and GitHub Actions.

## Prerequisites

- The GitHub repository has immutable releases enabled before any release tag is pushed.
- The local checkout is on `main`, clean, and up to date with `origin/main`.
- The release version is a new `v*` tag that has never been pushed.

## Validation

Run the full local gate before tagging:

    gofmt -w $(find . -name '*.go' -print)
    go test ./...
    go vet ./...
    go run github.com/goreleaser/goreleaser/v2@v2.15.4 check
    go run github.com/goreleaser/goreleaser/v2@v2.15.4 release --snapshot --clean
    npx --yes --package renovate renovate-config-validator renovate.json
    actionlint .github/workflows/*.yml
    git diff --check

If `actionlint` is not installed locally, install it or verify the workflow syntax in CI before tagging.

## Cut a Release

Create a signed or annotated tag from the verified commit:

    git tag -a v0.0.3 -m "v0.0.3"
    git push origin v0.0.3

The `Release` workflow runs GoReleaser on pushed `v*` tags. GoReleaser is configured not to replace existing release artifacts. If a release needs correction, create a new version tag.

## Verify

After the workflow completes, verify the GitHub release contains:

- `yard_Darwin_x86_64.tar.gz`
- `yard_Darwin_arm64.tar.gz`
- `yard_Linux_x86_64.tar.gz`
- `yard_Linux_arm64.tar.gz`
- `checksums.txt`

Then confirm the tag and release point at the intended commit.
