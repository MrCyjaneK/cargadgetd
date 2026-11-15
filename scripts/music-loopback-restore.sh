#!/usr/bin/env bash
# Put phone audio back after a killed cargadgetd loopback.
# Pulse may have adopted sink.null as the default sink; that survives SIGKILL
# and needs this (or a reboot). Safe to run anytime as defaultuser.
set -euo pipefail
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/100000}"
PACTL="${PACTL:-pactl}"
HW_SINK="${HW_SINK:-sink.primary_output}"
HW_SOURCE="${HW_SOURCE:-source.primary_input}"
NULL_SINK="${NULL_SINK:-sink.null}"
VOLFILE="${VOLFILE:-$XDG_RUNTIME_DIR/cargadgetd-music-volume}"

"$PACTL" info >/dev/null

if "$PACTL" list short sinks | awk '{print $2}' | grep -qx "$HW_SINK"; then
  "$PACTL" set-default-sink "$HW_SINK"
fi
if "$PACTL" list short sources | awk '{print $2}' | grep -qx "$HW_SOURCE"; then
  "$PACTL" set-default-source "$HW_SOURCE"
fi

vol=""
mute=""
if [[ -f "$VOLFILE" ]]; then
  vol="$(sed -n 1p "$VOLFILE" || true)"
  mute="$(sed -n 2p "$VOLFILE" || true)"
fi

nid="$("$PACTL" list short sinks | awk -v n="$NULL_SINK" '$2==n{print $1; exit}')"
if [[ -n "${nid:-}" ]]; then
  "$PACTL" list short sink-inputs | awk -v n="$nid" '$2==n{print $1}' | while read -r id; do
    "$PACTL" move-sink-input "$id" "$HW_SINK" || true
    if [[ -n "${vol:-}" ]]; then
      # unquoted $vol: "50% 50%" must be two pactl args
      # shellcheck disable=SC2086
      "$PACTL" set-sink-input-volume "$id" $vol || true
    fi
    if [[ -n "${mute:-}" ]]; then
      "$PACTL" set-sink-input-mute "$id" "$mute" || true
    fi
  done
fi

rm -f "$VOLFILE"
pkill -f 'parec.*cargadgetd-music' 2>/dev/null || true
echo "pulse: default sink=$("$PACTL" info | awk -F': ' '/Default Sink:/{print $2}')"
