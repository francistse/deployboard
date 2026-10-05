# Releasing Deployboard

Three public install paths share **one** tagged release:

| Path | What users run | What this repo maintains |
|---|---|---|
| **Homebrew** | `brew install --cask francistse/tap/deployboard` | GoReleaser `homebrew_casks` → `francistse/homebrew-tap` |
| **install.sh (from clone)** | `bash install.sh` or `bash install.sh --from-release` | LaunchAgent + config migration |
| **GitHub Release binary** | `curl …/install-release.sh \| bash` or download the `.tar.gz` | Darwin amd64/arm64 archives on the GitHub Release |

Do not invent a fourth distribution channel without updating this file.

## One-time setup (maintainer)

1. Create a public GitHub repo **`francistse/homebrew-tap`** (empty README is fine). Homebrew short-name: `francistse/tap`.
2. Create a fine-grained PAT with **Contents: Read and write** on `francistse/homebrew-tap` only.
3. Add it as repo secret **`HOMEBREW_TAP_TOKEN`** on `francistse/deployboard`.
4. Confirm [`.github/workflows/release.yml`](../.github/workflows/release.yml) is on `main`.

Without the tap + token, a tag still publishes **GitHub Release binaries**; the Homebrew cask push is skipped (`skip_upload` in [`.goreleaser.yml`](../.goreleaser.yml)).

## Cut a release

1. Land the changes on `main` (CI green: `make vet && make test`, `npm test`, landing lint/build if touched).
2. Update [`CHANGELOG.md`](../CHANGELOG.md) — move Unreleased notes under `## [X.Y.Z]`.
3. Bump visible version strings if needed (`Makefile` `VERSION`, landing `brand.version`, `PRODUCT-STATE.md`).
4. Tag and push:

```sh
git checkout main
git pull origin main
git tag -a vX.Y.Z -m "Deployboard vX.Y.Z"
git push origin vX.Y.Z
```

5. Watch the **Release** workflow. It should:
   - run Go tests (GoReleaser `before` hook)
   - upload `deployboard_X.Y.Z_darwin_{amd64,arm64}.tar.gz` + `checksums.txt`
   - push `Casks/deployboard.rb` to `francistse/homebrew-tap` (when `HOMEBREW_TAP_TOKEN` is set)

6. Smoke-test on a Mac:

```sh
# Homebrew
brew update
brew install --cask francistse/tap/deployboard
deployboard --version

# Release binary + LaunchAgent
curl -fsSL https://raw.githubusercontent.com/francistse/deployboard/main/install-release.sh | bash
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9410/
```

7. Confirm the GitHub Release body and assets look right; edit notes if needed.

## Re-run / fix a broken release

- **Workflow dispatch**: Actions → Release → Run workflow → optional tag (e.g. `v0.0.1`) to rebuild assets for an existing tag.
- If GitHub refuses to replace assets, delete the Release (keep the tag) and re-run, or bump to `vX.Y.Z+1`.
- If the cask did not update: check the workflow log for tap auth errors, then confirm `HOMEBREW_TAP_TOKEN` and that `francistse/homebrew-tap` exists.

## What not to do

- Do **not** point GoReleaser at `RoboZephyr/homebrew-tap` or tell users to `brew install RoboZephyr/tap/launch-pilot`.
- Do **not** put secrets in `.goreleaser.yml`, cask files, or docs.
- Do **not** move dependency installs into a per-boot script — releases are tag-driven only.
- Prefer **not** to hand-edit the published cask in the tap; the next tag overwrites it. Fix [`.goreleaser.yml`](../.goreleaser.yml) instead.

## Local dry-run

```sh
goreleaser release --snapshot --clean --skip=publish
# or: goreleaser check
```

Snapshot builds prove the archive layout; they do not publish.
