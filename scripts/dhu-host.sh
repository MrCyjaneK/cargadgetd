# Shared host helpers for Desktop Head Unit. Source this; do not execute it.
# Linux and macOS. DHU itself is the SDK extra desktop-head-unit.

dhu_host_find() {
  if [[ -n "${DHU_DIR:-}" && -x "${DHU_DIR}/desktop-head-unit" ]]; then
    return 0
  fi
  DHU_DIR=""
  local cand
  for cand in \
    ${ANDROID_SDK_ROOT:+"$ANDROID_SDK_ROOT/extras/google/auto"} \
    ${ANDROID_HOME:+"$ANDROID_HOME/extras/google/auto"} \
    ${ANDROID_HOME:+"$ANDROID_HOME/Sdk/extras/google/auto"} \
    "$HOME/Android/Sdk/extras/google/auto" \
    "$HOME/Library/Android/sdk/extras/google/auto"
  do
    if [[ -x "$cand/desktop-head-unit" ]]; then
      DHU_DIR="$cand"
      return 0
    fi
  done
  return 1
}

# GNU timeout's 124. Uses timeout, gtimeout, or python3.
dhu_run_timeout() {
  local secs="$1"
  shift
  if command -v timeout >/dev/null 2>&1; then
    timeout "$secs" "$@"
    return
  fi
  if command -v gtimeout >/dev/null 2>&1; then
    gtimeout "$secs" "$@"
    return
  fi
  # -c, not a heredoc: the caller's stdin is the DHU fifo (or a pipe).
  python3 -c '
import subprocess, sys
secs = float(sys.argv[1])
p = subprocess.Popen(sys.argv[2:])
try:
    rc = p.wait(secs)
except subprocess.TimeoutExpired:
    p.kill()
    p.wait()
    sys.exit(124)
sys.exit(rc if rc is not None else 1)
' "$secs" "$@"
}

# dhu_usb_present VID PID — hex, no 0x. True when that device is on the host bus.
dhu_usb_present() {
  local vid="$1" pid="$2"
  if command -v lsusb >/dev/null 2>&1; then
    lsusb -d "${vid}:${pid}" 2>/dev/null | grep -q .
    return
  fi
  if [[ "$(uname -s)" == Darwin ]]; then
    python3 - "$vid" "$pid" <<'PY'
import subprocess, sys
want_v = int(sys.argv[1], 16)
want_p = int(sys.argv[2], 16)
try:
    out = subprocess.check_output(
        ["ioreg", "-r", "-c", "IOUSBHostDevice", "-l", "-w0"],
        text=True, errors="replace")
except (OSError, subprocess.CalledProcessError):
    sys.exit(1)

def num(line):
    val = line.split("=", 1)[1].strip().rstrip(",")
    return int(val, 0)

block = []
def hit(lines):
    v = p = None
    for line in lines:
        if '"idVendor"' in line:
            v = num(line)
        elif '"idProduct"' in line:
            p = num(line)
    return v == want_v and p == want_p

for line in out.splitlines():
    if "IOUSBHostDevice" in line and ("+-o " in line or "+-o" in line):
        if block and hit(block):
            sys.exit(0)
        block = [line]
    else:
        block.append(line)
sys.exit(0 if block and hit(block) else 1)
PY
    return
  fi
  return 1
}

dhu_usb_list_18d1() {
  if command -v lsusb >/dev/null 2>&1; then
    lsusb -d 18d1: 2>/dev/null || true
    return
  fi
  if [[ "$(uname -s)" == Darwin ]]; then
    ioreg -r -c IOUSBHostDevice -l -w0 2>/dev/null | python3 -c '
import sys
block=[]
def show(lines):
    vals={}
    for line in lines:
        for key in ("idVendor","idProduct","USB Product Name","USB Vendor Name"):
            if "\"%s\""%key in line and "=" in line:
                vals[key]=line.split("=",1)[1].strip().rstrip(",")
    if vals.get("idVendor") in ("6353","0x18d1"):
        print("  "+" ".join(vals.get(k,"") for k in ("idVendor","idProduct","USB Vendor Name","USB Product Name")))
for line in sys.stdin:
    if "IOUSBHostDevice" in line:
        if block: show(block)
        block=[line]
    else:
        block.append(line)
if block: show(block)
' || true
  fi
}
