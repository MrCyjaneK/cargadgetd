#!/usr/bin/env bash
# Run Google Desktop Head Unit against a USB Android Auto phone.
#
#   ./scripts/start-dhu.sh
#   SOAK_SECS=90 ./scripts/start-dhu.sh
#   DHU_CONFIG=config/default.ini ./scripts/start-dhu.sh
#
# Do not pass a short --usb=SERIAL. DHU skips any device whose name is
# under 19 bytes, which drops the Jolla C2 serial. Plain --usb scans
# every device; the C2 answers AOAP from the usb-moded ask dialog.
#
# This laptop's Goodix fingerprint reader answers GET_PROTOCOL with garbage.
# DHU then blocks on it (control timeout is unlimited) after it has already
# switched the phone, so the projection window never comes up. Hide that
# device for the run and put it back on the way out.
set -euo pipefail

# shellcheck source=dhu-host.sh
source "$(cd "$(dirname "$0")" && pwd)/dhu-host.sh"
if ! dhu_host_find; then
  echo "start-dhu: desktop-head-unit not found (set DHU_DIR)" >&2
  exit 1
fi

# Linux DHU 2.x needs libc++ next to the binary. The macOS build is Mach-O
# and loads its own libusb from the same directory. Android Studio's LLDB
# tree ships libc++abi.so; the loader wants the SONAME libc++abi.so.1.
if [[ "$(uname -s)" == Linux ]]; then
  SDK="$(cd "$DHU_DIR/../../.." && pwd)"
  libpath="$DHU_DIR"
  if [[ -d "$SDK/emulator/lib64" ]]; then
    libpath="$libpath:$SDK/emulator/lib64"
  fi
  abi=""
  for cand in "$SDK/emulator/lib64/libc++abi.so.1" \
    /snap/android-studio/current/plugins/android-ndk/resources/lldb/lib64/libc++abi.so.1 \
    /snap/android-studio/current/plugins/android-ndk/resources/lldb/lib64/libc++abi.so
  do
    if [[ -f "$cand" ]]; then
      abi="$cand"
      break
    fi
  done
  if [[ -z "$abi" ]]; then
    abi="$(find /snap/android-studio -path '*/lldb/lib64/libc++abi.so*' -print 2>/dev/null | head -1 || true)"
  fi
  if [[ -n "$abi" && "$(basename "$abi")" != libc++abi.so.1 ]]; then
    ln -sfn "$abi" "$DHU_DIR/libc++abi.so.1"
  elif [[ -n "$abi" ]]; then
    libpath="$libpath:$(dirname "$abi")"
  fi
  export LD_LIBRARY_PATH="${libpath}${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
  export DISPLAY="${DISPLAY:-:0}"
fi

CFG="${DHU_CONFIG:-$DHU_DIR/config/default_720p.ini}"
if [[ ! -f "$CFG" && -f "$DHU_DIR/$DHU_CONFIG" ]]; then
  CFG="$DHU_DIR/$DHU_CONFIG"
fi
if [[ ! -f "$CFG" ]]; then
  echo "start-dhu: config not found: $CFG" >&2
  exit 1
fi

# Re-enumeration creates a new /dev/bus/usb node, often root:root 0664.
# libusb needs write. Keep relaxing mode while DHU runs.
# Host autosuspend mid-session is a gadget SUSPEND: the phone drops the TLS
# session and, if it cannot bounce the UDC, DHU never re-handshakes.
pin_phone_usb() {
  local d vid
  for d in /sys/bus/usb/devices/[0-9]*; do
    [[ -f "$d/idVendor" ]] || continue
    vid="$(cat "$d/idVendor" 2>/dev/null || true)"
    case "$vid" in
      0e8d|18d1)
        echo on | sudo -n tee "$d/power/control" >/dev/null 2>&1 || true
        echo -1 | sudo -n tee "$d/power/autosuspend" >/dev/null 2>&1 || true
        echo -1 | sudo -n tee "$d/power/autosuspend_delay_ms" >/dev/null 2>&1 || true
        ;;
    esac
  done
}

chmod_loop() {
  while true; do
    sudo -n chmod a+rw /dev/bus/usb/*/* 2>/dev/null || true
    pin_phone_usb
    sleep 0.2
  done
}
# Linux usbfs nodes come up root:root after AOA re-enumeration. macOS libusb
# opens the device through IOKit and has no /dev/bus/usb to chmod.
if [[ -d /dev/bus/usb ]] && ! sudo -n true 2>/dev/null; then
  echo "start-dhu: sudo is required to open USB device nodes" >&2
  exit 1
fi

chmod_pid=""
hold_pid=""
fifo=""
goodix_ports=()
cleanup() {
  trap - EXIT INT TERM
  [[ -n "$chmod_pid" ]] && kill "$chmod_pid" 2>/dev/null || true
  [[ -n "$hold_pid" ]] && kill "$hold_pid" 2>/dev/null || true
  [[ -n "$fifo" ]] && rm -f "$fifo"
  local p
  for p in "${goodix_ports[@]+"${goodix_ports[@]}"}"; do
    echo 0 | sudo -n tee "$p/disable" >/dev/null || true
  done
  [[ -n "$chmod_pid" ]] && wait "$chmod_pid" 2>/dev/null || true
  [[ -n "$hold_pid" ]] && wait "$hold_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# This laptop's Goodix reader answers GET_PROTOCOL with garbage. It is a
# Linux USB device; Macs do not have it on the bus DHU scans.
if [[ -d /sys/bus/usb/devices ]]; then
  for d in /sys/bus/usb/devices/[0-9]*; do
    [[ -f "$d/idVendor" && -f "$d/idProduct" ]] || continue
    if [[ "$(cat "$d/idVendor")" == 27c6 && "$(cat "$d/idProduct")" == 609c ]]; then
      port="$(readlink -f "$d/port")"
      echo "start-dhu: disabling fingerprint reader port ${port##*/}"
      echo 1 | sudo -n tee "$port/disable" >/dev/null
      goodix_ports+=("$port")
    fi
  done
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if ! dhu_usb_present 27c6 609c; then
      break
    fi
    sleep 0.2
  done
  if dhu_usb_present 27c6 609c; then
    echo "start-dhu: fingerprint reader is still on the bus; Head Unit would stall on it" >&2
    exit 1
  fi
fi

if [[ -d /dev/bus/usb ]]; then
  chmod_loop &
  chmod_pid=$!
fi

fifo="$(mktemp -u /tmp/dhu-start.XXXXXX)"
mkfifo "$fifo"
# RDWR so the writer does not block before DHU opens the read end.
python3 -c 'import os, time; os.open("'"$fifo"'", os.O_RDWR); time.sleep(86400)' &
hold_pid=$!

echo "start-dhu: $DHU_DIR/desktop-head-unit -c $CFG --usb"
if [[ -n "${SOAK_SECS:-}" ]]; then
  dhu_run_timeout "$SOAK_SECS" "$DHU_DIR/desktop-head-unit" -c "$CFG" --usb <"$fifo"
else
  "$DHU_DIR/desktop-head-unit" -c "$CFG" --usb <"$fifo"
fi
