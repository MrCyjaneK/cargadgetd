# cgshell

## Load

`shell.id` (default `basic`) selects `libcargadgetd_shell_<id>.so` in `/usr/lib/cargadgetd/plugins/`.

## Calls

The daemon calls these. Required: `init`, `draw`, `touch`. Strings and icons are only valid for the duration of the call.

```c
// after _init function returns app should respond to _draw and should not stall at any other calls
void cargadgetd_shell_<id>_init(int frame_ms, int width, int height);

// After init returns, once per app, after that app has loaded.
void cargadgetd_shell_<id>_app(const char *id, const char *name,
    const uint8_t *icon, int icon_w, int icon_h, int icon_stride,
    void (*draw)(uint32_t *rgba, int w, int h, int stride),
    void (*touch)(int x, int y, int action), // 0 up, 1 down, 2 move
    void (*pause)(void),
    void (*resume)(void));

// Gets called whenever there is new notification
void cargadgetd_shell_<id>_notif(const char *title, const char *body, const uint8_t *icon, int icon_w, int icon_h, int icon_stride);

void cargadgetd_shell_<id>_draw(uint32_t *rgba, int w, int h, int stride, int64_t now_unix_ms);
void cargadgetd_shell_<id>_touch(int x, int y, int action);
void cargadgetd_shell_<id>_huinput(const char *event, int pressed);
void cargadgetd_shell_<id>_gps_fix(int has_pos, double lat, double lon, double acc_m,
    int has_speed, int kmh, int has_course, double course, const char *err); // optional
void cargadgetd_shell_<id>_gps_sat(int gain, const char *kicker, const char *title,
    const char *sub, const char *detail, const char *flag); // optional; one-shot
void cargadgetd_shell_<id>_stop(void);
```

Optional settings use the same suffixes as an app (`settings_count`, `settings_label`, `settings_type`, `settings_enum_values`, get/set bool, enum, and int, `settings_error`). Missing `settings_count` means the shell has none. The host does not dlsym them. A settings UI does, and it does not need `init`.