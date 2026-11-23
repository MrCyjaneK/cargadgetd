#include "cg_app.h"

#include <dlfcn.h>
#include <stdio.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

extern void cgGoPlay(const int16_t *pcm, int nframes, int channels, int sample_rate);

static void host_play(const int16_t *pcm, int nframes, int channels, int sample_rate)
{
	cgGoPlay(pcm, nframes, channels, sample_rate);
}

static struct cargadgetd_host g_host;

void cg_host_set_frame_ms(int ms)
{
	if (ms <= 0)
		ms = 33;
	else if (ms < 16)
		ms = 16;
	g_host.play = host_play;
	g_host.frame_ms = ms;
}

struct cargadgetd_host *cg_host_get(void)
{
	g_host.play = host_play;
	if (g_host.frame_ms <= 0)
		g_host.frame_ms = 33;
	return &g_host;
}

void *cg_dlopen(const char *path, char **err)
{
	void *h = dlopen(path, RTLD_NOW | RTLD_GLOBAL);
	if (!h && err)
		*err = (char *)dlerror();
	return h;
}

struct cg_syms {
	void (*init)(const struct cargadgetd_host *);
	void (*draw)(uint32_t *, int, int, int);
	void (*touch)(int, int, int);
	void (*pause)(void);
	void (*resume)(void);
	void (*stop)(void);
	const char *(*name)(void);
	const uint8_t *(*icon)(int *, int *);
};

static void *app_dlsym(void *h, const char *app, const char *method)
{
	char buf[192];
	int n = snprintf(buf, sizeof buf, "cargadgetd_app_%s_%s", app, method);
	if (n < 0 || n >= (int)sizeof buf)
		return 0;
	dlerror();
	return dlsym(h, buf);
}

struct cg_syms *cg_load_syms(void *h, const char *app, char **err)
{
	struct cg_syms *out;

	if (!h || !app || !app[0]) {
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
	out->draw = (void (*)(uint32_t *, int, int, int))app_dlsym(h, app, "draw");
	if (!out->draw) {
		free(out);
		if (err)
			*err = "missing cargadgetd_app_<id>_draw";
		return 0;
	}
	out->init = (void (*)(const struct cargadgetd_host *))app_dlsym(h, app, "init");
	out->touch = (void (*)(int, int, int))app_dlsym(h, app, "touch");
	out->pause = (void (*)(void))app_dlsym(h, app, "pause");
	out->resume = (void (*)(void))app_dlsym(h, app, "resume");
	out->stop = (void (*)(void))app_dlsym(h, app, "stop");
	out->name = (const char *(*)(void))app_dlsym(h, app, "name");
	out->icon = (const uint8_t *(*)(int *, int *))app_dlsym(h, app, "icon");
	return out;
}

void cg_syms_free(struct cg_syms *s)
{
	free(s);
}

void cg_call_init(struct cg_syms *s, const struct cargadgetd_host *h)
{
	if (s && s->init)
		s->init(h);
}

void cg_call_stop(struct cg_syms *s)
{
	if (s && s->stop)
		s->stop();
}

const char *cg_sym_name(struct cg_syms *s)
{
	const char *n;

	if (!s || !s->name)
		return "";
	n = s->name();
	return n ? n : "";
}

const uint8_t *cg_sym_icon(struct cg_syms *s, int *w, int *h)
{
	if (!s || !s->icon) {
		if (w)
			*w = 0;
		if (h)
			*h = 0;
		return 0;
	}
	return s->icon(w, h);
}

void *cg_huinput_sym(void *h, const char *app, const char *event)
{
	char buf[192];
	int n;

	if (!h || !app || !event || !event[0])
		return 0;
	n = snprintf(buf, sizeof buf, "cargadgetd_app_%s_huinput_%s", app, event);
	if (n < 0 || n >= (int)sizeof buf)
		return 0;
	dlerror();
	return dlsym(h, buf);
}

void cg_call_huinput_fn(void *fn, int pressed)
{
	if (fn)
		((void (*)(int))fn)(pressed);
}

static void *cg_app_sym(void *h, const char *app, const char *method)
{
	char buf[192];
	int n;

	if (!h || !app || !method)
		return 0;
	n = snprintf(buf, sizeof buf, "cargadgetd_app_%s_%s", app, method);
	if (n < 0 || n >= (int)sizeof buf)
		return 0;
	dlerror();
	return dlsym(h, buf);
}

void *cg_nowplaying_sym(void *h, const char *app)
{
	return cg_app_sym(h, app, "nowplaying");
}

int cg_call_nowplaying(void *fn, struct cargadgetd_nowplaying *out)
{
	int (*f)(struct cargadgetd_nowplaying *);

	if (!fn || !out)
		return 0;
	f = fn;
	return f(out);
}

void *cg_gps_fix_sym(void *h, const char *app)
{
	return cg_app_sym(h, app, "gps_fix");
}

void *cg_gps_sat_sym(void *h, const char *app)
{
	return cg_app_sym(h, app, "gps_sat");
}

void cg_call_gps_fix(void *fn, int has_pos, double lat, double lon, double acc_m,
	int has_speed, int kmh, int has_course, double course, const char *err)
{
	void (*f)(int, double, double, double, int, int, int, double, const char *);

	if (!fn)
		return;
	f = fn;
	f(has_pos, lat, lon, acc_m, has_speed, kmh, has_course, course, err);
}

void cg_call_gps_sat(void *fn, int gain, const char *kicker, const char *title,
	const char *sub, const char *detail, const char *flag)
{
	void (*f)(int, const char *, const char *, const char *, const char *, const char *);

	if (!fn)
		return;
	f = fn;
	f(gain, kicker, title, sub, detail, flag);
}

static void cg_noop_touch(int x, int y, int action)
{
	(void)x;
	(void)y;
	(void)action;
}

static void cg_noop_void(void)
{
}

void *cg_fn_draw(struct cg_syms *s)
{
	return s ? (void *)s->draw : 0;
}

void *cg_fn_touch(struct cg_syms *s)
{
	if (!s)
		return 0;
	if (s->touch)
		return (void *)s->touch;
	return (void *)cg_noop_touch;
}

void *cg_fn_pause(struct cg_syms *s)
{
	if (!s)
		return 0;
	if (s->pause)
		return (void *)s->pause;
	return (void *)cg_noop_void;
}

void *cg_fn_resume(struct cg_syms *s)
{
	if (!s)
		return 0;
	if (s->resume)
		return (void *)s->resume;
	return (void *)cg_noop_void;
}
