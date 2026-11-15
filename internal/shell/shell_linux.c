#include "cg_shell.h"

#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void *cg_shell_dlopen(const char *path, char **err)
{
	void *h = dlopen(path, RTLD_NOW | RTLD_GLOBAL);
	if (!h && err)
		*err = (char *)dlerror();
	return h;
}

struct cg_shell_syms {
	void (*init)(int, int, int);
	void (*app)(const char *, const char *, const uint8_t *, int, int, int,
		cargadgetd_app_draw_fn, cargadgetd_app_touch_fn,
		cargadgetd_app_ctrl_fn, cargadgetd_app_ctrl_fn);
	void (*notif)(const char *, const char *, const uint8_t *, int, int, int);
	void (*draw)(uint32_t *, int, int, int, int64_t);
	void (*touch)(int, int, int);
	void (*huinput)(const char *, int);
	void (*gps_fix)(int, double, double, double, int, int, int, double, const char *);
	void (*gps_sat)(int, const char *, const char *, const char *, const char *, const char *);
	void (*stop)(void);
};

static void *shell_dlsym(void *h, const char *id, const char *method)
{
	char buf[192];
	int n = snprintf(buf, sizeof buf, "cargadgetd_shell_%s_%s", id, method);
	if (n < 0 || n >= (int)sizeof buf)
		return 0;
	dlerror();
	return dlsym(h, buf);
}

struct cg_shell_syms *cg_shell_load(void *h, const char *id, char **err)
{
	struct cg_shell_syms *out;

	if (!h || !id || !id[0]) {
		if (err)
			*err = "invalid args";
		return 0;
	}
	out = calloc(1, sizeof(*out));
	if (!out) {
		if (err)
			*err = "oom";
		return 0;
	}
	out->init = (void (*)(int, int, int))shell_dlsym(h, id, "init");
	out->draw = (void (*)(uint32_t *, int, int, int, int64_t))shell_dlsym(h, id, "draw");
	out->touch = (void (*)(int, int, int))shell_dlsym(h, id, "touch");
	if (!out->init || !out->draw || !out->touch) {
		free(out);
		if (err)
			*err = "missing cargadgetd_shell_<id>_{init,draw,touch}";
		return 0;
	}
	out->app = (void (*)(const char *, const char *, const uint8_t *, int, int, int,
		cargadgetd_app_draw_fn, cargadgetd_app_touch_fn,
		cargadgetd_app_ctrl_fn, cargadgetd_app_ctrl_fn))shell_dlsym(h, id, "app");
	out->notif = (void (*)(const char *, const char *, const uint8_t *, int, int, int))shell_dlsym(h, id, "notif");
	out->huinput = (void (*)(const char *, int))shell_dlsym(h, id, "huinput");
	out->gps_fix = (void (*)(int, double, double, double, int, int, int, double, const char *))shell_dlsym(h, id, "gps_fix");
	out->gps_sat = (void (*)(int, const char *, const char *, const char *, const char *, const char *))shell_dlsym(h, id, "gps_sat");
	out->stop = (void (*)(void))shell_dlsym(h, id, "stop");
	return out;
}

void cg_shell_init(struct cg_shell_syms *s, int frame_ms, int w, int h)
{
	if (s && s->init)
		s->init(frame_ms, w, h);
}

void cg_shell_app(struct cg_shell_syms *s, const char *id, const char *name,
	const uint8_t *icon, int iw, int ih, int istride,
	void *draw, void *touch, void *pause, void *resume)
{
	if (s && s->app)
		s->app(id, name, icon, iw, ih, istride,
			(cargadgetd_app_draw_fn)draw,
			(cargadgetd_app_touch_fn)touch,
			(cargadgetd_app_ctrl_fn)pause,
			(cargadgetd_app_ctrl_fn)resume);
}

void cg_shell_notif(struct cg_shell_syms *s, const char *title, const char *body,
	const uint8_t *icon, int iw, int ih, int istride)
{
	if (s && s->notif)
		s->notif(title, body, icon, iw, ih, istride);
}

void cg_shell_draw(struct cg_shell_syms *s, uint32_t *rgba, int w, int h, int stride, int64_t now_ms)
{
	if (s && s->draw)
		s->draw(rgba, w, h, stride, now_ms);
}

void cg_shell_touch(struct cg_shell_syms *s, int x, int y, int action)
{
	if (s && s->touch)
		s->touch(x, y, action);
}

void cg_shell_huinput(struct cg_shell_syms *s, const char *event, int pressed)
{
	if (s && s->huinput)
		s->huinput(event, pressed);
}

void cg_shell_gps_fix(struct cg_shell_syms *s, int has_pos, double lat, double lon, double acc_m,
	int has_speed, int kmh, int has_course, double course, const char *err)
{
	if (s && s->gps_fix)
		s->gps_fix(has_pos, lat, lon, acc_m, has_speed, kmh, has_course, course, err);
}

void cg_shell_gps_sat(struct cg_shell_syms *s, int gain, const char *kicker, const char *title,
	const char *sub, const char *detail, const char *flag)
{
	if (s && s->gps_sat)
		s->gps_sat(gain, kicker, title, sub, detail, flag);
}

void cg_shell_stop(struct cg_shell_syms *s)
{
	if (s && s->stop)
		s->stop();
}
