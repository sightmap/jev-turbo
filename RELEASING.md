# Releasing

Versions come from changesets. Every user-visible change adds a file under
`.changeset/` (`npm run changeset`). On push to `main`, `release.yml` opens or
updates a "Version Packages" PR that bumps `npm/package.json` and writes
`npm/CHANGELOG.md`. Merging that PR tags `vX.Y.Z`; GoReleaser builds six
platform binaries into a GitHub release, and the npm job publishes
`@sightmap/jev-turbo-<os>-<arch>` plus the `@sightmap/jev-turbo` meta package,
which lists them as optional dependencies so `npm install` fetches only the
matching binary. The meta package also depends on `@sightmap/sightmap`, so
`npx @sightmap/jev-turbo` has the `sightmap` binary next to it for
`browser start`.

## One-time npm setup

npm trusted publishing (OIDC) is configured per package, and a package must
exist before a Trusted Publisher can be registered on it. So the first version
is published by hand by a maintainer of the `@sightmap` scope:

```bash
git checkout v0.1.0
goreleaser release --snapshot --clean          # or download the archives from the GitHub release
node npm/scripts/build-npm-packages.mjs --version 0.1.0 --dist dist --out npm-staging
for d in npm-staging/@sightmap/*/; do (cd "$d" && npm publish); done
(cd npm-staging/meta && npm publish)
```

Then, on npmjs.com, add a Trusted Publisher to each of the seven packages:
GitHub Actions, repository `sightmap/jev-turbo`, workflow `release.yml`,
environment blank. Every later release publishes from CI with no token.

## Checking a release

Run `npx -y @sightmap/jev-turbo version` from a directory outside this repo.
Inside the repo, npm resolves the name to the `npm/` workspace package, which
has no platform binary, and the shim reports "no prebuilt binary found".

## Go module

The module is `github.com/sightmap/jev-turbo` at the repo root, so the same
`vX.Y.Z` tag serves `go install github.com/sightmap/jev-turbo/cmd/jev-turbo@vX.Y.Z`.
It depends on `github.com/sightmap/sightmap/go`, which is tagged `go/vX.Y.Z` in
its own repo; bump it in `go.mod` when a new sightmap ships.
