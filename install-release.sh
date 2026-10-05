#!/usr/bin/env bash
# install-release.sh — install Deployboard from a GitHub Release without cloning
# or installing Go. Downloads install.sh + config.example.json from the same ref,
# fetches the darwin archive for this machine, then runs install.sh --binary.
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/francistse/deployboard/main/install-release.sh | bash
#
# Pin a version / pass install.sh flags:
#   curl -fsSL .../install-release.sh | bash -s -- --from-release v0.0.2 --port 9410
#   VERSION=v0.0.2 bash install-release.sh --no-agent
#
# Prefer Homebrew when you have it:
#   brew install --cask francistse/tap/deployboard

set -euo pipefail

GITHUB_REPO="${DEPLOYBOARD_GITHUB_REPO:-francistse/deployboard}"
GITHUB_REF="${DEPLOYBOARD_GITHUB_REF:-main}"
RAW_BASE="https://raw.githubusercontent.com/${GITHUB_REPO}/${GITHUB_REF}"

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/deployboard-install-release.XXXXXX")"
cleanup() { rm -rf "${WORKDIR}"; }
trap cleanup EXIT

echo "Fetching installer from ${RAW_BASE}..."
curl -fsSL "${RAW_BASE}/install.sh" -o "${WORKDIR}/install.sh"
curl -fsSL "${RAW_BASE}/config.example.json" -o "${WORKDIR}/config.example.json"
chmod +x "${WORKDIR}/install.sh"

# Default to --from-release latest unless the caller already passed --from-release
# or --binary. VERSION=vX.Y.Z is a convenience alias for --from-release.
args=("$@")
has_source=false
for a in "${args[@]+"${args[@]}"}"; do
	case "${a}" in
		--from-release|--binary) has_source=true; break ;;
	esac
done

if [[ -n ${VERSION:-} ]]; then
	args=(--from-release "${VERSION}" "${args[@]+"${args[@]}"}")
elif [[ ${has_source} == false ]]; then
	args=(--from-release latest "${args[@]+"${args[@]}"}")
fi

echo "Running install.sh ${args[*]}..."
bash "${WORKDIR}/install.sh" "${args[@]}"
