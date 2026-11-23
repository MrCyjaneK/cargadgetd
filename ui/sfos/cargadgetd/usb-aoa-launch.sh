#!/bin/sh
# Wait until a host sends Android Auto AOA START, then bind the accessory
# gadget and start the MD. gadgetctl -aoa stops usb-moded for that wait so
# a cable insert cannot rewrite the gadget, and clears the UDC as soon as
# START is seen. usb-moded is restored when the waiter exits or the session ends.
#
# systemd runs this with --fg. A manual start double-forks so the caller
# is not blocked.
set -u

GADGET=/usr/share/cargadgetd/bin/cargadgetd-gadget
MD=/usr/share/cargadgetd/bin/cargadgetd-md
RESTORE=/usr/share/cargadgetd/music-loopback-restore.sh
USER_NAME=defaultuser
UID_NUM=100000
MD_LOG=/tmp/cargadgetd.log
SELF_LOG=/tmp/cargadgetd-aoa.log

if [ "${1:-}" != --fg ]; then
	nohup "$0" --fg >>"$SELF_LOG" 2>&1 &
	exit 0
fi

log() { echo "$(date '+%Y-%m-%dT%H:%M:%S') $*"; }

# daemon.transport from the user YAML. Missing, or anything other than
# wireless or wpp, is usb.
transport_mode() {
	f="/home/${USER_NAME}/.config/cargadgetd/config.yaml"
	[ -f "$f" ] || {
		echo usb
		return
	}
	awk '
		/^daemon:/ { in_d = 1; next }
		in_d && /^[^ #]/ { in_d = 0 }
		in_d && $1 == "transport:" {
			v = tolower($2)
			gsub(/"/, "", v)
			gsub(/\047/, "", v)
			if (v == "wireless") print "wireless"
			else if (v == "wpp") print "wpp"
			else print "usb"
			found = 1
			exit
		}
		END { if (!found) print "usb" }
	' "$f"
}

# One restore, then leave. Exit 78 is RestartPreventExitStatus, so systemd
# does not start this waiter again. A poll here would wake the CPU forever.
leave_wireless() {
	log "transport wireless - USB off, exiting"
	touch "$MD_LOG"
	chown "${USER_NAME}:${USER_NAME}" "$MD_LOG" 2>/dev/null || true
	"$GADGET" -usb-off >>"$MD_LOG" 2>&1 || true
	exit 78
}

# Accessory gadget stays down. Bluetooth and Wi-Fi are left as the user set
# them. The MD blocks on the system bus until BlueZ signals, and this waiter
# blocks in the foreground until that process exits. Exit 78 is only the
# wireless path.
run_wpp() {
	log "transport wpp - USB off"
	touch "$MD_LOG"
	chown "${USER_NAME}:${USER_NAME}" "$MD_LOG" 2>/dev/null || true
	"$GADGET" -usb-off >>"$MD_LOG" 2>&1 || true
	kept=$(rotate_log || true)
	touch "$MD_LOG"
	chown "${USER_NAME}:${USER_NAME}" "$MD_LOG" 2>/dev/null || true
	if [ -n "$kept" ]; then
		log "kept previous log as $kept" | tee -a "$MD_LOG"
	fi
	kill_md
	log "=== MD start wpp ===" | tee -a "$MD_LOG"
	start=$(date +%s)
	su -s /bin/sh "$USER_NAME" -c "
		export HOME=/home/${USER_NAME}
		export XDG_RUNTIME_DIR=/run/user/${UID_NUM}
		export GLIBC_TUNABLES=glibc.rtld.optional_static_tls=65536
		cd /usr/share/cargadgetd/bin
		exec '$MD' >>'$MD_LOG' 2>&1
	"
	now=$(date +%s)
	# A crash exits immediately. Sleep here so the outer loop cannot
	# respawn it in a tight cycle. A long run is a finished session.
	if [ $((now - start)) -lt 20 ]; then
		log "MD exited quickly - sleeping" | tee -a "$MD_LOG"
		sleep 30
	fi
}

# Rename a non-empty live log to cargadgetd-YYYYMMDD-HHMMSS.log.
# The live path stays put so the UI and droid traces keep appending to it.
# Keeps the newest 30 archives. Does not touch cargadgetd-unhandled.log.
rotate_log() {
	[ -s "$MD_LOG" ] || return 0
	dir=$(dirname "$MD_LOG")
	base=$(basename "$MD_LOG" .log)
	ts=$(date '+%Y%m%d-%H%M%S')
	dest="$dir/${base}-${ts}.log"
	n=0
	while [ -e "$dest" ]; do
		n=$((n + 1))
		dest="$dir/${base}-${ts}-${n}.log"
	done
	mv "$MD_LOG" "$dest" || return 0
	ls -1t "$dir/${base}"-[0-9]*.log 2>/dev/null | awk 'NR>30' | while read -r old; do
		rm -f "$old"
	done
	printf '%s\n' "$dest"
}

# busybox pgrep -x does not see cargadgetd-md from this unit, and the
# nohup pid can exit while the daemon keeps running. /proc comm is the check.
md_alive() {
	for c in /proc/[0-9]*/comm; do
		[ -r "$c" ] || continue
		if [ "$(cat "$c" 2>/dev/null)" = "cargadgetd-md" ]; then
			return 0
		fi
	done
	return 1
}

# Session teardown can die inside a plugin before music Halt runs, which
# leaves the player on sink.null. The restore script is safe if Pulse is
# already back on the hardware sink.
restore_audio() {
	[ -x "$RESTORE" ] || return 0
	su -s /bin/sh "$USER_NAME" -c "
		export HOME=/home/${USER_NAME}
		export XDG_RUNTIME_DIR=/run/user/${UID_NUM}
		'$RESTORE'
	" >>"$MD_LOG" 2>&1 || true
}

# Link pins /lipstick/orientationLock to landscape and stamps the previous
# value. SIGKILL skips the plugin's restore, so put it back here.
ORIENT_STAMP=/run/user/${UID_NUM}/cargadgetd-link-orient
restore_orient() {
	[ -f "$ORIENT_STAMP" ] || return 0
	prev=$(head -n 1 "$ORIENT_STAMP" 2>/dev/null || true)
	case "$prev" in
	dynamic|portrait|portrait-inverted|landscape|landscape-inverted) ;;
	*) rm -f "$ORIENT_STAMP"; return 0 ;;
	esac
	su -s /bin/sh "$USER_NAME" -c "
		export HOME=/home/${USER_NAME}
		export XDG_RUNTIME_DIR=/run/user/${UID_NUM}
		export DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/${UID_NUM}/dbus/user_bus_socket
		dconf write /lipstick/orientationLock \"'${prev}'\"
	" >>"$MD_LOG" 2>&1 || true
	rm -f "$ORIENT_STAMP"
}

kill_md() {
	# SIGTERM is not enough when the previous MD is blocked in an ep2 read
	# and still holds the control port. The next start then exits with
	# "address already in use" and nobody reads the new accessory.
	pkill -9 -f '/usr/share/cargadgetd/bin/cargadgetd-md' 2>/dev/null || true
	i=0
	while pgrep -f '/usr/share/cargadgetd/bin/cargadgetd-md' >/dev/null 2>&1; do
		i=$((i + 1))
		if [ "$i" -gt 20 ]; then
			break
		fi
		sleep 0.1
	done
}

log "wait for AOA start"

gp=0
trap 'if [ "$gp" -ne 0 ]; then kill "$gp" 2>/dev/null || true; wait "$gp" 2>/dev/null || true; fi; if [ "${started:-0}" -eq 1 ]; then restore_audio; fi; if [ "$(transport_mode)" != wireless ]; then kill_md; fi; restore_orient; exit 0' TERM INT

# Stay up across a teardown. Exiting here made systemd restart the unit,
# and the MD's gadget -stop then killed the new waiter until the start
# limit left nobody listening for the next START.
while true; do
	mode=$(transport_mode)
	if [ "$mode" = "wireless" ]; then
		leave_wireless
	fi
	if [ "$mode" = "wpp" ]; then
		run_wpp
		continue
	fi
	kept=$(rotate_log || true)
	touch "$MD_LOG"
	chown "${USER_NAME}:${USER_NAME}" "$MD_LOG" 2>/dev/null || true
	if [ -n "$kept" ]; then
		log "kept previous log as $kept" | tee -a "$MD_LOG"
	fi
	"$GADGET" -aoa >>"$MD_LOG" 2>&1 &
	gp=$!
	started=0
	md_seen=0
	md_miss=0
	while kill -0 "$gp" 2>/dev/null; do
		if [ "$started" -eq 0 ] && [ -e /dev/usb-ffs/aa/ep1 ]; then
			ready=0
			for u in /sys/kernel/config/usb_gadget/*/UDC; do
				v=$(cat "$u" 2>/dev/null || true)
				if [ -n "$v" ]; then
					ready=1
				fi
			done
			if [ "$ready" -eq 1 ]; then
				kill_md
				log "=== MD start ===" | tee -a "$MD_LOG"
				# maps.so carries ~4.7KB of initial-exec TLS from MapLibre.
				# glibc's dlopen surplus is a few hundred bytes, so the
				# plugin is skipped and Neon falls back to its own map.
				md_pid=$(su -s /bin/sh "$USER_NAME" -c "
					export HOME=/home/${USER_NAME}
					export XDG_RUNTIME_DIR=/run/user/${UID_NUM}
					export GLIBC_TUNABLES=glibc.rtld.optional_static_tls=65536
					cd /usr/share/cargadgetd/bin
					nohup '$MD' >>'$MD_LOG' 2>&1 &
					echo \$!
				")
				log "MD pid ${md_pid:-unknown}" | tee -a "$MD_LOG"
				started=1
				md_seen=0
				md_miss=0
			fi
		fi
		# DHU exit does not drop VBUS. If the MD process is gone, restore
		# the ask gadget. A missing daemon for the first 8s is still
		# startup; after it has been seen, 1.6s is enough.
		if [ "$started" -eq 1 ]; then
			if md_alive; then
				md_seen=1
				md_miss=0
			else
				md_miss=$((md_miss + 1))
				limit=40
				if [ "$md_seen" -eq 1 ]; then
					limit=8
				fi
				if [ "$md_miss" -ge "$limit" ]; then
					log "MD not running - restoring USB"
					kill "$gp" 2>/dev/null || true
					break
				fi
			fi
		fi
		sleep 0.2
	done
	wait "$gp" 2>/dev/null || true
	gp=0
	log "gadget exited; waiting for the next AOA start"
	if [ "$started" -eq 1 ]; then
		restore_audio
	fi
	kill_md
	restore_orient
	sleep 1
done
