#!/bin/bash
# Tests for the launcher's takeover of Omarchy's screensaver switch and for
# launching, against a throwaway home and stand-ins for Hyprland, the
# terminal and Omarchy's commands. Run from anywhere: tests/launcher.sh

# The stand-in commands below are written in single quotes on purpose.
# shellcheck disable=SC2016

set -u
LAUNCHER=$(readlink -f "$(dirname "$0")/../bin/koi-screensaver")
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
fails=0
pass() { printf '  ok    %s\n' "$1"; }
fail() { printf '  FAIL  %s\n' "$1"; fails=$((fails + 1)); }
expect() { if [[ $2 == "$3" ]]; then pass "$1"; else fail "$1 (got '$2', want '$3')"; fi; }

export HOME=$T/home XDG_STATE_HOME=$T/home/.local/state XDG_RUNTIME_DIR=$T/run
mkdir -p "$HOME" "$XDG_RUNTIME_DIR"
FLAG=$HOME/.local/state/omarchy/toggles/screensaver-off
MARK=$XDG_STATE_HOME/koi-screensaver/owns-screensaver-off
reset() { rm -f "$FLAG" "$MARK"; }
claim() { "$LAUNCHER" claim "$1"; }
release() { "$LAUNCHER" release "$1"; }
flag() { [[ -f $FLAG ]] && echo on || echo off; }

MOCK=$T/mock
mkdir -p "$MOCK"
mock() { printf '#!/bin/bash\n%s\n' "$2" >"$MOCK/$1"; chmod +x "$MOCK/$1"; }
mock pgrep 'exit 1'
mock pkill 'echo "pkill $*" >>"$LOG"'
mock omarchy-hyprland-monitor-focused 'echo DP-1'
mock omarchy-notification-send 'echo "${@: -1}" >>"$NOTES"'
mock xdg-terminal-exec 'echo "${TERMINAL_ID:-com.mitchellh.ghostty.desktop}"'
mock omarchy-shell 'echo "${LOCKED:-false}"'
export PATH=$MOCK:$PATH HYPRLAND_INSTANCE_SIGNATURE=test
echo "Taking over Omarchy's screensaver switch"
reset; claim old >/dev/null; claim new >/dev/null; release old
expect "a late release from the old instance after a reload leaves the new one's" "$(flag)" on
reset; claim old >/dev/null; release old; claim new >/dev/null
expect "release then claim across a reload" "$(flag)" on
reset; claim a >/dev/null; release a; claim a >/dev/null; release a; claim a >/dev/null
expect "rapid on, off, on" "$(flag)" on
reset; mkdir -p "$(dirname "$FLAG")"; : >"$FLAG"
expect "a flag you set yourself is yours" "$(claim a)" user-off
release a
expect "and is kept when the koi let go" "$(flag)" on
reset; claim a >/dev/null; rm "$FLAG"; sleep 0.01; : >"$FLAG"; release a
expect "you switched it off again yourself while the koi held it: kept" "$(flag)" on
reset; claim a >/dev/null; rm "$FLAG"
expect "you switched Omarchy's back on while the koi held it: they step aside" "$(claim a)" user-on
expect "and it stays on" "$(flag)" off
reset
for i in $(seq 20); do claim "t$i" >/dev/null & done; wait
read -r holder _ <"$MARK"
expect "twenty claims at once leave one flag" "$(flag)" on
release "$holder"
expect "which its holder can release" "$(flag)" off
reset; claim a >/dev/null
expect "a koi that can't run here give Omarchy's back" "$(TERMINAL_ID=xterm claim a)" "problem: it needs Alacritty, Foot, Ghostty or Kitty as the terminal"
expect "and it is on again" "$(flag)" off
reset; mkdir -p "$(dirname "$FLAG")"; : >"$FLAG"
TERMINAL_ID=xterm claim a >/dev/null
expect "but a flag you set yourself stays" "$(flag)" on
reset; claim gone >/dev/null; "$LAUNCHER" retire gone
expect "an instance going away gives it back" "$(flag)" off
expect "and a claim of its arriving late is refused" "$(claim gone)" retired
expect "leaving it on" "$(flag)" off
expect "a bad token is refused" "$("$LAUNCHER" claim 'a;b' 2>&1)" "bad token"

echo "Launching"
# Hyprland's event stream: a window event only once the launcher opens one.
# DISMISS=early: you come back as soon as the first window shows (before the
# second is asked for); DISMISS=late: while the second is on its way;
# NOOPEN=1: the terminal never shows a window.
mock socat 'exec tail -n +1 -f "$EVENTS_FILE"'
mock hyprctl 'if [[ $1 == monitors ]]; then echo "[{\"name\":\"DP-1\",\"refreshRate\":60},{\"name\":\"DP-2\",\"refreshRate\":60}]"; exit; fi
echo "$*" >>"$LOG"
[[ $* == *exec_cmd* && -z ${NOOPEN:-} ]] || exit 0
n=$(grep -c exec_cmd "$LOG")
echo "openwindow>>w$n,1,org.omarchy.screensaver,koi" >>"$EVENTS_FILE"
if [[ ${DISMISS:-} == early && $n == 1 ]] || [[ ${DISMISS:-} == late && $n == 2 ]]; then echo "closewindow>>w1" >>"$EVENTS_FILE"; fi'
export LOG=$T/hyprctl.log NOTES=$T/notes EVENTS_FILE=$T/events
launches() { grep -c exec_cmd "$LOG" 2>/dev/null; true; }
run() { : >"$LOG"; : >"$NOTES"; : >"$EVENTS_FILE"; "$LAUNCHER"; launches; }
closed() { grep -c '^pkill' "$LOG"; true; }

expect "one window on each of two monitors" "$(run)" 2
expect "dismissing the first stops the second from opening" "$(DISMISS=early run)" 1
expect "dismissed while the second was on its way: it opens" "$(DISMISS=late run)" 2
expect "and is closed again at once" "$(closed)" 1
expect "nothing opens over the lock screen" "$(LOCKED=true run)" 0
expect "nothing opens when the lock state is unknown" "$(LOCKED=maybe run)" 0
: >"$LOG"; : >"$EVENTS_FILE"
for i in $(seq 20); do "$LAUNCHER" & done; wait
expect "twenty starts at once open one window per monitor" "$(launches)" 2
expect "an unsupported terminal opens nothing" "$(TERMINAL_ID=xterm run)" 0
expect "and says why" "$(cat "$NOTES")" "Koi Screensaver can't run: it needs Alacritty, Foot, Ghostty or Kitty as the terminal"
expect "a window that never appears stops the launch" "$(NOOPEN=1 run)" 1
expect "and says so" "$(cat "$NOTES")" "Koi Screensaver couldn't open its window"
: >"$LOG"; : >"$EVENTS_FILE"
NOOPEN=1 "$LAUNCHER" & pid=$!
sleep 0.5; kill -TERM "$pid"; start=$SECONDS; wait "$pid"
expect "stopped by the plugin while waiting for a window: it opens no more" "$(launches)" 1
expect "and stops at once" "$(( SECONDS - start < 2 ))" 1
expect "and the next start isn't blocked" "$(run)" 2
expect "check reports a problem" "$(TERMINAL_ID=xterm "$LAUNCHER" check)" "it needs Alacritty, Foot, Ghostty or Kitty as the terminal"
expect "check says ok when it can run" "$("$LAUNCHER" check)" ok

echo
if ((fails)); then echo "$fails failed"; exit 1; fi
echo "all passed"
