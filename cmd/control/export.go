package main

/*
#include <stdlib.h>
*/
import "C"

import "github.com/cargadgetd/cargadgetd/internal/control"

func cString(s string) *C.char { return C.CString(s) }

func cBool(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

//export cargadgetd_internal_md_running
func cargadgetd_internal_md_running() C.int { return cBool(control.MDRunning()) }

//export cargadgetd_internal_ffs_ready
func cargadgetd_internal_ffs_ready() C.int { return cBool(control.FFSReady()) }

//export cargadgetd_internal_aoa_running
func cargadgetd_internal_aoa_running() C.int { return cBool(control.AOARunning()) }

//export cargadgetd_internal_aoa_start
func cargadgetd_internal_aoa_start() *C.char { return cString(control.AoaStart()) }

//export cargadgetd_internal_aoa_stop
func cargadgetd_internal_aoa_stop() *C.char { return cString(control.AoaStop()) }

//export cargadgetd_internal_aoa_restart
func cargadgetd_internal_aoa_restart() *C.char { return cString(control.AoaRestart()) }

//export cargadgetd_internal_log_text
func cargadgetd_internal_log_text() *C.char { return cString(control.LogText()) }

//export cargadgetd_internal_log_clear
func cargadgetd_internal_log_clear() { control.LogClear() }

//export cargadgetd_internal_certs_ready
func cargadgetd_internal_certs_ready() C.int { return cBool(control.CertsReady()) }

//export cargadgetd_internal_certs_probe
func cargadgetd_internal_certs_probe() *C.char { return cString(control.CertsProbe()) }

//export cargadgetd_internal_certs_download
func cargadgetd_internal_certs_download() *C.char { return cString(control.CertsDownload()) }

//export cargadgetd_internal_certs_install
func cargadgetd_internal_certs_install(apk *C.char) *C.char {
	return cString(control.CertsInstall(C.GoString(apk)))
}

//export cargadgetd_internal_onboarding_done
func cargadgetd_internal_onboarding_done() C.int { return cBool(control.OnboardingDone()) }

//export cargadgetd_internal_addons_present
func cargadgetd_internal_addons_present() C.int { return cBool(control.AddonsPresent()) }

//export cargadgetd_internal_addons_fetch
func cargadgetd_internal_addons_fetch() *C.char { return cString(control.AddonsFetch()) }

//export cargadgetd_internal_addons_refresh
func cargadgetd_internal_addons_refresh() *C.char { return cString(control.AddonsRefresh()) }

//export cargadgetd_internal_addons_count
func cargadgetd_internal_addons_count() C.int { return C.int(control.AddonsCount()) }

//export cargadgetd_internal_addons_group
func cargadgetd_internal_addons_group(i C.int) *C.char {
	row, ok := control.AddonAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(row.Group)
}

//export cargadgetd_internal_addons_title
func cargadgetd_internal_addons_title(i C.int) *C.char {
	row, ok := control.AddonAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(row.Title)
}

//export cargadgetd_internal_addons_description
func cargadgetd_internal_addons_description(i C.int) *C.char {
	row, ok := control.AddonAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(row.Description)
}

//export cargadgetd_internal_addons_icon
func cargadgetd_internal_addons_icon(i C.int) *C.char {
	row, ok := control.AddonAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(row.Icon)
}

//export cargadgetd_internal_addons_installed
func cargadgetd_internal_addons_installed(i C.int) C.int {
	row, ok := control.AddonAt(int(i))
	if !ok {
		return 0
	}
	return cBool(row.Installed)
}

//export cargadgetd_internal_addons_update
func cargadgetd_internal_addons_update(i C.int) C.int {
	row, ok := control.AddonAt(int(i))
	if !ok {
		return 0
	}
	return cBool(row.Update)
}

//export cargadgetd_internal_addons_url
func cargadgetd_internal_addons_url(i C.int) *C.char {
	row, ok := control.AddonAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(row.URL)
}

//export cargadgetd_internal_apk_start
func cargadgetd_internal_apk_start() *C.char { return cString(control.ApkStart()) }

//export cargadgetd_internal_apk_list
func cargadgetd_internal_apk_list(path *C.char) C.int {
	return C.int(control.ApkList(C.GoString(path)))
}

//export cargadgetd_internal_apk_count
func cargadgetd_internal_apk_count() C.int { return C.int(control.ApkCount()) }

//export cargadgetd_internal_apk_name
func cargadgetd_internal_apk_name(i C.int) *C.char {
	row, ok := control.ApkAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(row.Name)
}

//export cargadgetd_internal_apk_path
func cargadgetd_internal_apk_path(i C.int) *C.char {
	row, ok := control.ApkAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(row.Path)
}

//export cargadgetd_internal_apk_dir
func cargadgetd_internal_apk_dir(i C.int) C.int {
	row, ok := control.ApkAt(int(i))
	if !ok {
		return 0
	}
	return cBool(row.Dir)
}

//export cargadgetd_internal_encoder_count
func cargadgetd_internal_encoder_count() C.int { return C.int(control.EncoderCount()) }

//export cargadgetd_internal_encoder_id
func cargadgetd_internal_encoder_id(i C.int) *C.char {
	id, _, ok := control.EncoderAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(id)
}

//export cargadgetd_internal_encoder_title
func cargadgetd_internal_encoder_title(i C.int) *C.char {
	_, title, ok := control.EncoderAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(title)
}

//export cargadgetd_internal_transport_count
func cargadgetd_internal_transport_count() C.int { return C.int(control.TransportCount()) }

//export cargadgetd_internal_transport_id
func cargadgetd_internal_transport_id(i C.int) *C.char {
	m, ok := control.TransportAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(m.ID)
}

//export cargadgetd_internal_transport_title
func cargadgetd_internal_transport_title(i C.int) *C.char {
	m, ok := control.TransportAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(m.Title)
}

//export cargadgetd_internal_transport_body
func cargadgetd_internal_transport_body(i C.int) *C.char {
	m, ok := control.TransportAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(m.Body)
}

//export cargadgetd_internal_transport_hint
func cargadgetd_internal_transport_hint(i C.int) *C.char {
	m, ok := control.TransportAt(int(i))
	if !ok {
		return cString("")
	}
	return cString(m.Hint)
}

//export cargadgetd_internal_transport_needs_peer
func cargadgetd_internal_transport_needs_peer(i C.int) C.int {
	m, ok := control.TransportAt(int(i))
	if !ok {
		return 0
	}
	return cBool(m.NeedsPeer)
}

//export cargadgetd_internal_transport_require_peer
func cargadgetd_internal_transport_require_peer(i C.int) C.int {
	m, ok := control.TransportAt(int(i))
	if !ok {
		return 0
	}
	return cBool(m.RequirePeer)
}

//export cargadgetd_internal_bench_start
func cargadgetd_internal_bench_start(encoder *C.char) *C.char {
	return cString(control.BenchStart(C.GoString(encoder)))
}

//export cargadgetd_internal_bench_running
func cargadgetd_internal_bench_running() C.int { return cBool(control.BenchRunning()) }

//export cargadgetd_internal_bench_status
func cargadgetd_internal_bench_status() *C.char { return cString(control.BenchStatus()) }

//export cargadgetd_internal_bench_result
func cargadgetd_internal_bench_result() *C.char { return cString(control.BenchResult()) }

//export cargadgetd_internal_bench_watch
func cargadgetd_internal_bench_watch() { control.BenchWatch() }

//export cargadgetd_internal_bench_clear
func cargadgetd_internal_bench_clear() { control.BenchClear() }

func main() {}
