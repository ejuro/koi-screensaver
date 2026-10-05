# Security

Koi is a **visual screensaver only**. Locking, authentication and the lock timeout belong to Omarchy. Koi does not implement a security boundary and should not be used as a replacement for the session lock.

## Trust and access

Omarchy plugins run unsandboxed as the logged-in user. Review the QML, launcher and Go source before enabling this plugin. Koi launches terminal windows, changes its own inline settings in `shell.json`, temporarily owns the stock screensaver-off flag, and temporarily hides the compositor cursor. Its window cleanup uses Omarchy's screensaver command-line class convention, which can also match the stock screensaver or other same-user commands containing that class.

Koi's runtime code makes no network requests, sends no telemetry, and invokes no sudo/pkexec commands. Git installation and updates may use the network. A build may download Go and its pinned module dependency if unavailable locally. The bundled executables can be rebuilt with `./build.sh`; their Go and x/sys licenses accompany them in [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).

Plugin-owned persistent data is under `${XDG_STATE_HOME:-$HOME/.local/state}`: the `koi-screensaver/` ownership directory and `koi-screensaver.log`. The log records exit reasons and input categories, not typed keys. `KOI_SCREENSAVER_LOG` can redirect it. Runtime locks use `$XDG_RUNTIME_DIR`; the current fallback is `/tmp`, so run the plugin in a normal Omarchy desktop with its private runtime directory set. Host settings/state paths and all read/write locations are documented in the [README](README.md#what-it-runs-and-touches).

## Cleanup and known limits

Normal unload attempts to release only the stock-off flag identified by the current ownership record. A crash, SIGKILL or power loss can skip that cleanup and cursor restoration. Removal while the shell is stopped cannot run the service's destruction handler. Follow the README's [crash recovery instructions](README.md#recovery-after-a-crash); do not remove a flag the user or another active plugin deliberately owns.

The launcher serializes startup and refuses new windows when its lock-state query is unknown. That does not prove every lock/startup race is resolved. Already-open windows may remain after disabling the plugin and need dismissal. Multi-monitor timing, suspend/resume, terminal failures and adverse idle-to-lock transitions need further live coverage. The tested versions and the scope of the single-monitor Ghostty smoke check are listed in the [requirements](README.md#requirements); other terminals and aarch64 were not runtime-validated in that check.

## Reporting a concern

The maintainer is [ejuro](https://github.com/ejuro). If the repository's **Security → Report a vulnerability** option is available, use that private channel. Otherwise ask the maintainer for a private reporting route without posting exploit details, credentials, logs containing personal data, or sensitive screenshots in a public issue. Non-sensitive bugs can use the repository's issue tracker when available.

Include the exact plugin commit, affected Omarchy/Quickshell/Hyprland and terminal versions, expected and observed behavior, and a minimal reproduction. State whether the problem affects the visual saver, state cleanup, or the session lock. No response-time guarantee or independent security certification is claimed.
