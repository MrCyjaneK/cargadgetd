package main

/*
#cgo LDFLAGS: -ldl
#include <stdlib.h>
*/
import "C"
import "unsafe"

var (
	daemonName   = C.CString("Cargadgetd")
	settingsHold []*C.char
)

func holdC(s string) *C.char {
	p := C.CString(s)
	settingsHold = append(settingsHold, p)
	if len(settingsHold) > 32 {
		C.free(unsafe.Pointer(settingsHold[0]))
		settingsHold = settingsHold[1:]
	}
	return p
}

//export cargadgetd_app_daemon_name
func cargadgetd_app_daemon_name() *C.char { return daemonName }

//export cargadgetd_app_daemon_settings_count
func cargadgetd_app_daemon_settings_count() C.int { return C.int(coreSettingsCount()) }

//export cargadgetd_app_daemon_settings_label
func cargadgetd_app_daemon_settings_label(i C.int) *C.char {
	return holdC(coreSettingsLabel(int(i)))
}

//export cargadgetd_app_daemon_settings_type
func cargadgetd_app_daemon_settings_type(i C.int) C.int {
	return C.int(coreSettingsType(int(i)))
}

//export cargadgetd_app_daemon_settings_enum_values
func cargadgetd_app_daemon_settings_enum_values(i C.int) *C.char {
	return holdC(coreSettingsEnums(int(i)))
}

//export cargadgetd_app_daemon_settings_get_bool
func cargadgetd_app_daemon_settings_get_bool(i C.int) C.int {
	return C.int(coreSettingsGetBool(int(i)))
}

//export cargadgetd_app_daemon_settings_get_enum
func cargadgetd_app_daemon_settings_get_enum(i C.int) *C.char {
	return holdC(coreSettingsGetEnum(int(i)))
}

//export cargadgetd_app_daemon_settings_get_int
func cargadgetd_app_daemon_settings_get_int(i C.int) C.int {
	return C.int(coreSettingsGetInt(int(i)))
}

//export cargadgetd_app_daemon_settings_get_string
func cargadgetd_app_daemon_settings_get_string(i C.int) *C.char {
	return holdC(coreSettingsGetString(int(i)))
}

//export cargadgetd_app_daemon_settings_set_bool
func cargadgetd_app_daemon_settings_set_bool(i, v C.int) C.int {
	return C.int(coreSettingsSetBool(int(i), int(v)))
}

//export cargadgetd_app_daemon_settings_set_enum
func cargadgetd_app_daemon_settings_set_enum(i C.int, v *C.char) C.int {
	return C.int(coreSettingsSetEnum(int(i), C.GoString(v)))
}

//export cargadgetd_app_daemon_settings_set_int
func cargadgetd_app_daemon_settings_set_int(i, v C.int) C.int {
	return C.int(coreSettingsSetInt(int(i), int(v)))
}

//export cargadgetd_app_daemon_settings_set_string
func cargadgetd_app_daemon_settings_set_string(i C.int, v *C.char) C.int {
	return C.int(coreSettingsSetString(int(i), C.GoString(v)))
}

//export cargadgetd_app_daemon_settings_error
func cargadgetd_app_daemon_settings_error() *C.char {
	return holdC(coreSettingsError())
}

func main() {}
