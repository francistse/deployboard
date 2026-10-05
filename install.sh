#!/usr/bin/env bash
# install.sh — one-command macOS install for Deployboard.
# Idempotent. On an existing install it retires the previous LaunchAgent
# (com.deployboard.launch-pilot), moves ~/.config/launch-pilot/ when
# ~/.config/deployboard/ does not already exist, and leaves a one-release
# symlink at $PREFIX/launch-pilot. No sudo. Prints each migration it performs.
# --dry-run does not write under ~/Library/LaunchAgents.

set -euo pipefail

DEFAULT_PREFIX="${HOME}/bin"
DEFAULT_PORT=9410
OLD_CONFIG_DIR="${HOME}/.config/launch-pilot"
NEW_CONFIG_DIR="${HOME}/.config/deployboard"
DEFAULT_CONFIG="${NEW_CONFIG_DIR}/config.json"
PLIST_LABEL="com.deployboard.agent"
OLD_LABEL="com.deployboard.launch-pilot"
LOG_DIR="${HOME}/Library/Logs"
LOG_FILE="${LOG_DIR}/deployboard.log"
BINARY_NAME="deployboard"
LEGACY_BINARY_NAME="launch-pilot"

GITHUB_REPO="${DEPLOYBOARD_GITHUB_REPO:-francistse/deployboard}"

usage() {
	cat <<EOF
Usage: $(basename "$0") [options]

Options:
  --prefix <dir>       Install binary to <dir> (default: ${DEFAULT_PREFIX})
  --port <n>           Listen port (default: ${DEFAULT_PORT}, 0 = random)
  --config <file>      Path to config.json (default: ${DEFAULT_CONFIG})
  --binary <file>      Use pre-built binary instead of building from source
  --from-release [tag] Download a GitHub Release binary (default: latest) instead of building
  --no-agent           Do not create the LaunchAgent (run manually instead)
  --uninstall          Remove the LaunchAgent and plist; keep the binary
  --purge              With --uninstall, also remove the binary and the compatibility symlink
  --dry-run            Print what would be done and exit 0
  --help               Show this help

Without --binary / --from-release, builds from this checkout with Go.
For a curl one-liner that does not need a clone, use install-release.sh.
EOF
}

PREFIX="${DEFAULT_PREFIX}"
PORT="${DEFAULT_PORT}"
CONFIG_FILE="${DEFAULT_CONFIG}"
BINARY=""
FROM_RELEASE=""
NO_AGENT=false
UNINSTALL=false
PURGE=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
	case "$1" in
		--prefix) PREFIX="$2"; shift 2 ;;
		--port) PORT="$2"; shift 2 ;;
		--config) CONFIG_FILE="$2"; shift 2 ;;
		--binary) BINARY="$2"; shift 2 ;;
		--from-release)
			if [[ $# -ge 2 && "$2" != --* ]]; then
				FROM_RELEASE="$2"
				shift 2
			else
				FROM_RELEASE="latest"
				shift
			fi
			;;
		--no-agent) NO_AGENT=true; shift ;;
		--uninstall) UNINSTALL=true; shift ;;
		--purge) PURGE=true; shift ;;
		--dry-run) DRY_RUN=true; shift ;;
		--help) usage; exit 0 ;;
		*) echo "Unknown option: $1" >&2; usage; exit 1 ;;
	esac
done

if [[ -n ${BINARY} && -n ${FROM_RELEASE} ]]; then
	echo "Use either --binary or --from-release, not both." >&2
	exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
AGENTS_DIR="${HOME}/Library/LaunchAgents"
RELEASE_FETCH_DIR=""

cleanup_release_fetch() {
	if [[ -n ${RELEASE_FETCH_DIR} && -d ${RELEASE_FETCH_DIR} ]]; then
		rm -rf "${RELEASE_FETCH_DIR}"
	fi
}
trap cleanup_release_fetch EXIT

# Resolve a GitHub release tag + download the darwin archive for this machine.
# Sets BINARY to the extracted path. Archives are named by GoReleaser:
#   deployboard_<version>_darwin_<amd64|arm64>.tar.gz
fetch_release_binary() {
	local want="$1"
	local tag ver arch goarch url archive

	if [[ $(uname -s) != Darwin ]]; then
		echo "--from-release only works on macOS (got $(uname -s))." >&2
		exit 1
	fi
	arch="$(uname -m)"
	case "${arch}" in
		arm64|aarch64) goarch="arm64" ;;
		x86_64) goarch="amd64" ;;
		*)
			echo "Unsupported architecture: ${arch}" >&2
			exit 1
			;;
	esac

	if [[ ${want} == latest ]]; then
		echo "Resolving latest GitHub release for ${GITHUB_REPO}..."
		tag="$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
			| sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
		if [[ -z ${tag} ]]; then
			echo "Could not resolve latest release tag for ${GITHUB_REPO}." >&2
			echo "Publish darwin archives with the Release workflow first (see docs/RELEASE.md)." >&2
			exit 1
		fi
	else
		tag="${want}"
	fi
	ver="${tag#v}"
	archive="deployboard_${ver}_darwin_${goarch}.tar.gz"
	url="https://github.com/${GITHUB_REPO}/releases/download/${tag}/${archive}"

	RELEASE_FETCH_DIR="$(mktemp -d "${TMPDIR:-/tmp}/deployboard-release.XXXXXX")"
	echo "Downloading ${url}..."
	if ! curl -fsSL -o "${RELEASE_FETCH_DIR}/${archive}" "${url}"; then
		echo "Download failed. Does release ${tag} include ${archive}?" >&2
		echo "See docs/RELEASE.md — tag a release after the Release workflow is live." >&2
		exit 1
	fi
	tar -xzf "${RELEASE_FETCH_DIR}/${archive}" -C "${RELEASE_FETCH_DIR}"
	if [[ ! -f ${RELEASE_FETCH_DIR}/${BINARY_NAME} ]]; then
		echo "Archive ${archive} did not contain ${BINARY_NAME}." >&2
		exit 1
	fi
	chmod +x "${RELEASE_FETCH_DIR}/${BINARY_NAME}"
	# Clear quarantine when present (unsigned GitHub Release binaries).
	xattr -dr com.apple.quarantine "${RELEASE_FETCH_DIR}/${BINARY_NAME}" 2>/dev/null || true
	BINARY="${RELEASE_FETCH_DIR}/${BINARY_NAME}"
	echo "Using release ${tag} (${goarch})"
}

domain_for() {
	printf 'gui/%s/%s' "$(id -u)" "$1"
}

plist_for() {
	printf '%s/%s.plist' "${AGENTS_DIR}" "$1"
}

label_loaded() {
	launchctl print "$(domain_for "$1")" >/dev/null 2>&1
}

# mode "if-loaded": bootout + disable only when launchctl still has the label.
# mode "always": always attempt bootout + disable (uninstall / reinstall).
# The plist and any stale .plist.tmp are removed in both modes.
# Dry-run prints the steps and does not touch ~/Library/LaunchAgents.
retire_label() {
	local label="$1"
	local mode="$2"
	local plist tmp domain loaded
	plist="$(plist_for "${label}")"
	tmp="${plist}.tmp"
	domain="$(domain_for "${label}")"
	loaded=false
	if label_loaded "${label}"; then
		loaded=true
	fi
	if [[ ${loaded} == true || ${mode} == always ]]; then
		if [[ ${DRY_RUN} == true ]]; then
			echo "Would launchctl bootout ${domain}"
			echo "Would launchctl disable ${domain}"
		else
			if launchctl bootout "${domain}" 2>/dev/null; then
				echo "  bootout ${label}"
			else
				echo "  bootout: ${label} not loaded (ok)"
			fi
			launchctl disable "${domain}" 2>/dev/null || true
			echo "  disable ${label}"
			if [[ ${loaded} == true ]]; then
				local _
				for _ in $(seq 1 24); do
					label_loaded "${label}" || break
					sleep 0.25
				done
			fi
		fi
	fi
	if [[ -f ${plist} ]]; then
		if [[ ${DRY_RUN} == true ]]; then
			echo "Would remove ${plist}"
		else
			rm -f "${plist}"
			echo "  removed ${plist}"
		fi
	fi
	if [[ -e ${tmp} ]]; then
		if [[ ${DRY_RUN} == true ]]; then
			echo "Would remove stale ${tmp}"
		else
			rm -f "${tmp}"
			echo "  removed stale ${tmp}"
		fi
	fi
}

migrate_config_dir() {
	if [[ -d ${OLD_CONFIG_DIR} && ! -e ${NEW_CONFIG_DIR} ]]; then
		if [[ ${DRY_RUN} == true ]]; then
			echo "Would move ${OLD_CONFIG_DIR} -> ${NEW_CONFIG_DIR}"
		else
			mkdir -p "$(dirname "${NEW_CONFIG_DIR}")"
			mv "${OLD_CONFIG_DIR}" "${NEW_CONFIG_DIR}"
			echo "Migrated config directory ${OLD_CONFIG_DIR} -> ${NEW_CONFIG_DIR}"
		fi
	elif [[ -d ${OLD_CONFIG_DIR} && -e ${NEW_CONFIG_DIR} ]]; then
		echo "Keeping ${NEW_CONFIG_DIR} (already exists); left ${OLD_CONFIG_DIR} in place"
	fi
}

if [[ ${UNINSTALL} == true ]]; then
	echo "Uninstalling ${PLIST_LABEL}..."
	retire_label "${PLIST_LABEL}" always
	retire_label "${OLD_LABEL}" if-loaded
	if [[ ${PURGE} == true ]]; then
		if [[ ${DRY_RUN} == true ]]; then
			echo "Would remove ${PREFIX}/${BINARY_NAME}"
			echo "Would remove ${PREFIX}/${LEGACY_BINARY_NAME}"
		else
			rm -f "${PREFIX}/${BINARY_NAME}" "${PREFIX}/${LEGACY_BINARY_NAME}"
			echo "  removed ${PREFIX}/${BINARY_NAME}"
			echo "  removed ${PREFIX}/${LEGACY_BINARY_NAME}"
		fi
	fi
	if [[ ${DRY_RUN} == true ]]; then
		echo "dry-run: uninstall complete"
	fi
	exit 0
fi

# Build, fetch, or locate the binary before touching a running agent. A failed
# build/fetch leaves the previous install in place. Dry-run still compiles or
# downloads; it does not install.
if [[ -n ${FROM_RELEASE} ]]; then
	fetch_release_binary "${FROM_RELEASE}"
elif [[ -n ${BINARY} ]]; then
	if [[ ! -f ${BINARY} ]]; then
		echo "Binary not found: ${BINARY}" >&2
		exit 1
	fi
else
	if command -v go >/dev/null 2>&1; then
		echo "Building ${BINARY_NAME}..."
		( cd "${SCRIPT_DIR}" && make build )
		BINARY="${SCRIPT_DIR}/${BINARY_NAME}"
	else
		echo "Go not found; provide --binary <path>, --from-release, or install Go." >&2
		exit 1
	fi
fi

# Retire the previous label before installing the symlink or bootstrapping the
# new one, so the old agent cannot keep the port or restart onto the new binary.
echo "Checking previous LaunchAgent ${OLD_LABEL}..."
retire_label "${OLD_LABEL}" if-loaded

BIN_DEST="${PREFIX}/${BINARY_NAME}"
LEGACY_LINK="${PREFIX}/${LEGACY_BINARY_NAME}"
if [[ ${DRY_RUN} == true ]]; then
	echo "Would install binary to ${BIN_DEST}"
	echo "Would symlink ${LEGACY_LINK} -> ${BIN_DEST}"
else
	mkdir -p "${PREFIX}"
	install -m 0755 "${BINARY}" "${BIN_DEST}"
	echo "Installed ${BIN_DEST}"
	ln -sfn "${BIN_DEST}" "${LEGACY_LINK}"
	echo "Symlinked ${LEGACY_LINK} -> ${BIN_DEST}"
fi

migrate_config_dir

config_present=false
if [[ -f ${CONFIG_FILE} ]]; then
	config_present=true
elif [[ ${DRY_RUN} == true && -d ${OLD_CONFIG_DIR} && ! -e ${NEW_CONFIG_DIR} ]]; then
	case "${CONFIG_FILE}" in
		"${NEW_CONFIG_DIR}"/*)
			rel="${CONFIG_FILE#"${NEW_CONFIG_DIR}/"}"
			if [[ -f ${OLD_CONFIG_DIR}/${rel} ]]; then
				config_present=true
				echo "Config would be kept by moving ${OLD_CONFIG_DIR}"
			fi
			;;
	esac
fi

if [[ ${config_present} == false ]]; then
	if [[ ${DRY_RUN} == true ]]; then
		echo "Would create ${CONFIG_FILE} from defaults"
	else
		mkdir -p "$(dirname "${CONFIG_FILE}")"
		cp "${SCRIPT_DIR}/config.example.json" "${CONFIG_FILE}"
		echo "Created ${CONFIG_FILE}"
	fi
elif [[ ${DRY_RUN} == false || -f ${CONFIG_FILE} ]]; then
	echo "Config already exists at ${CONFIG_FILE}"
fi

if [[ ${DRY_RUN} == true ]]; then
	echo "Would ensure log directory ${LOG_DIR}"
else
	mkdir -p "${LOG_DIR}"
fi

if [[ ${NO_AGENT} == true ]]; then
	echo "Skipping LaunchAgent (--no-agent)"
	echo "Run: ${BIN_DEST} --config ${CONFIG_FILE} --port ${PORT}"
else
	# Lint a temp plist outside ~/Library/LaunchAgents. Dry-run must not create one there.
	plist_tmp="$(mktemp "${TMPDIR:-/tmp}/deployboard-agent.XXXXXX.plist")"
	cat > "${plist_tmp}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>${PLIST_LABEL}</string>
	<key>ProgramArguments</key>
	<array>
		<string>${BIN_DEST}</string>
		<string>--config</string>
		<string>${CONFIG_FILE}</string>
		<string>--port</string>
		<string>${PORT}</string>
	</array>
	<key>WorkingDirectory</key>
	<string>${HOME}</string>
	<key>StandardOutPath</key>
	<string>${LOG_FILE}</string>
	<key>StandardErrorPath</key>
	<string>${LOG_FILE}</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
EOF
	plutil -lint "${plist_tmp}" >/dev/null

	PLIST_PATH="$(plist_for "${PLIST_LABEL}")"
	if [[ ${DRY_RUN} == true ]]; then
		rm -f "${plist_tmp}"
		echo "Would write LaunchAgent to ${PLIST_PATH}"
		echo "Would bootstrap: launchctl bootstrap gui/$(id -u) ${PLIST_PATH}"
		health="$(curl -s -o /dev/null -w '%{http_code}' -m 3 "http://127.0.0.1:${PORT}/" || echo 000)"
		echo "Post-install health check (port ${PORT}): ${health}"
	else
		mkdir -p "${AGENTS_DIR}"
		mv "${plist_tmp}" "${PLIST_PATH}"
		# disable from a previous uninstall survives reboot, so enable before bootstrap.
		# bootout is async: wait until the label is gone, then retry once.
		new_domain="$(domain_for "${PLIST_LABEL}")"
		launchctl bootout "${new_domain}" 2>/dev/null || true
		launchctl enable "${new_domain}" 2>/dev/null || true
		for _ in $(seq 1 24); do
			label_loaded "${PLIST_LABEL}" || break
			sleep 0.25
		done
		if ! launchctl bootstrap "gui/$(id -u)" "${PLIST_PATH}" 2>/tmp/deployboard-bootstrap.err; then
			echo "bootstrap failed, retrying:" >&2
			cat /tmp/deployboard-bootstrap.err >&2 || true
			sleep 1
			launchctl bootstrap "gui/$(id -u)" "${PLIST_PATH}"
		fi
		echo "LaunchAgent ${PLIST_LABEL} loaded (RunAtLoad + KeepAlive)"

		sleep 1
		health="$(curl -s -o /dev/null -w '%{http_code}' -m 3 "http://127.0.0.1:${PORT}/" || echo 000)"
		if [[ "${health}" == "200" ]]; then
			echo "Dashboard: http://127.0.0.1:${PORT}/"
		else
			echo "Dashboard not yet responding (HTTP ${health}); check ${LOG_FILE}"
		fi
	fi
fi

cat <<'EOF'

=== Next step: Telegram alerts (optional) ===
Store the bot token in the macOS Keychain:

  security add-generic-password -U -s deployboard-telegram -a bot_token \
    -w "$TELEGRAM_BOT_TOKEN" -T /usr/bin/security

Or open the dashboard -> Settings and paste it there. The token is written
straight to the Keychain: never a file, never a log line, never returned to the
page. Set alerts.telegram.chat_id in the config file (or in Settings) as well.
EOF
