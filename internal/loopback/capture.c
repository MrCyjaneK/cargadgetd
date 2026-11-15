#define _GNU_SOURCE

#include <errno.h>
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#define MUSIC_FRAMES 2048
#define MUSIC_CHANS 2
#define MUSIC_RATE 48000
#define MUSIC_PCM_BYTES (MUSIC_FRAMES * MUSIC_CHANS * 2)

#if !defined(__linux__)
static int pthread_timedjoin_np(pthread_t th, void **ret, const struct timespec *ts)
{
	(void)ts;
	return pthread_join(th, ret);
}
#endif

static pthread_mutex_t cap_mu = PTHREAD_MUTEX_INITIALIZER;
static pthread_t cap_th;
static int cap_alive;
static volatile int cap_run;
static volatile int cap_fd = -1;
static FILE *cap_dump;
static int cap_dump_left;
static unsigned cap_chunks;
static char cap_dump_prefix[256];

extern void cgLoopbackPlay(const int16_t *pcm, int nframes, int channels, int rate);

void loopback_set_pcm_dump(const char *prefix)
{
	if (!prefix || !prefix[0]) {
		cap_dump_prefix[0] = 0;
		return;
	}
	snprintf(cap_dump_prefix, sizeof cap_dump_prefix, "%s", prefix);
}

static void cap_log_hz(const int16_t *s)
{
	int zc = 0, peak = 0, i;
	int16_t prev = s[0];

	for (i = 0; i < MUSIC_FRAMES; i++) {
		int16_t v = s[i * MUSIC_CHANS];
		int a = v < 0 ? -v : v;

		if (a > peak)
			peak = a;
		if ((prev < 0 && v >= 0) || (prev >= 0 && v < 0))
			zc++;
		prev = v;
	}
	fprintf(stderr, "loopback: cap L~%.0fHz zc=%d peak=%d (assume %dHz %dch)\n",
		(zc / 2.0) * MUSIC_RATE / (double)MUSIC_FRAMES, zc, peak,
		MUSIC_RATE, MUSIC_CHANS);
}

static void *loopback_cap_loop(void *arg)
{
	uint8_t buf[MUSIC_PCM_BYTES];
	int logged = 0;

	(void)arg;
	while (cap_run) {
		size_t got = 0;

		while (got < MUSIC_PCM_BYTES && cap_run) {
			ssize_t n = read(cap_fd, buf + got, MUSIC_PCM_BYTES - got);

			if (n < 0) {
				if (errno == EINTR)
					continue;
				goto out;
			}
			if (n == 0)
				goto out;
			got += (size_t)n;
		}
		if (!cap_run || got != MUSIC_PCM_BYTES)
			break;
		{
			const int16_t *pcm = (const int16_t *)buf;

			cap_chunks++;
			if (!logged || cap_chunks == 25 || (cap_chunks % 50) == 0)
				cap_log_hz(pcm);
			if (!logged) {
				logged = 1;
				fprintf(stderr, "loopback: pcm flowing\n");
			}
			if (cap_dump && cap_dump_left > 0) {
				int n = MUSIC_PCM_BYTES;

				if (n > cap_dump_left)
					n = cap_dump_left;
				n = (int)fwrite(buf, 1, (size_t)n, cap_dump);
				if (n > 0)
					cap_dump_left -= n;
				if (cap_dump_left <= 0) {
					fflush(cap_dump);
					fprintf(stderr, "loopback: cap dump done\n");
				}
			}
			cgLoopbackPlay(pcm, MUSIC_FRAMES, MUSIC_CHANS, MUSIC_RATE);
		}
	}
out:
	return 0;
}

void loopback_capture_stop(void)
{
	int fd;
	pthread_t th;
	int alive;

	pthread_mutex_lock(&cap_mu);
	cap_run = 0;
	fd = cap_fd;
	cap_fd = -1;
	alive = cap_alive;
	th = cap_th;
	cap_alive = 0;
	pthread_mutex_unlock(&cap_mu);
	if (fd >= 0)
		close(fd);
	if (alive) {
		struct timespec ts;

		clock_gettime(CLOCK_REALTIME, &ts);
		ts.tv_sec += 1;
		if (pthread_timedjoin_np(th, 0, &ts) != 0) {
			pthread_detach(th);
			fprintf(stderr, "loopback: capture thread stuck, detached\n");
		}
	}
	if (cap_dump) {
		fclose(cap_dump);
		cap_dump = 0;
		cap_dump_left = 0;
	}
}

void loopback_capture_start(int fd)
{
	loopback_capture_stop();
	if (fd < 0)
		return;
	pthread_mutex_lock(&cap_mu);
	cap_fd = fd;
	cap_run = 1;
	cap_chunks = 0;
	{
		if (cap_dump_prefix[0]) {
			char path[512];

			snprintf(path, sizeof path, "%s-cap.s16le", cap_dump_prefix);
			cap_dump = fopen(path, "wb");
			cap_dump_left = cap_dump ? 48000 * 2 * 2 * 2 : 0;
			if (cap_dump)
				fprintf(stderr, "loopback: dumping capture to %s\n", path);
		}
	}
	if (pthread_create(&cap_th, 0, loopback_cap_loop, 0) != 0) {
		cap_run = 0;
		cap_fd = -1;
		close(fd);
		pthread_mutex_unlock(&cap_mu);
		fprintf(stderr, "loopback: pthread_create failed\n");
		return;
	}
	cap_alive = 1;
	pthread_mutex_unlock(&cap_mu);
}

static void loopback_system(const char *cmd)
{
	int st = system(cmd);
	(void)st;
}

/* Best-effort Pulse heal if the process exits without Go Restore.
 * SIGKILL still needs scripts/music-loopback-restore.sh. */
__attribute__((destructor))
static void loopback_pulse_destructor(void)
{
	loopback_capture_stop();
	if (access("/usr/bin/pactl", X_OK) != 0)
		return;
	loopback_system("/usr/bin/pactl set-default-sink sink.primary_output >/dev/null 2>&1");
	loopback_system("/usr/bin/pactl set-default-source source.primary_input >/dev/null 2>&1");
	loopback_system("/bin/sh -c '"
		     "f=/run/user/100000/cargadgetd-music-volume; "
		     "nid=$(/usr/bin/pactl list short sinks 2>/dev/null | awk \"\\$2==\\\"sink.null\\\"{print \\$1; exit}\"); "
		     "[ -n \"$nid\" ] || exit 0; "
		     "vol=; mute=; "
		     "if [ -f \"$f\" ]; then vol=$(sed -n 1p \"$f\"); mute=$(sed -n 2p \"$f\"); rm -f \"$f\"; fi; "
		     "/usr/bin/pactl list short sink-inputs 2>/dev/null | awk -v n=\"$nid\" \"\\$2==n{print \\$1}\" | "
		     "while read id; do "
		     "/usr/bin/pactl move-sink-input \"$id\" sink.primary_output >/dev/null 2>&1; "
		     "[ -n \"$vol\" ] && /usr/bin/pactl set-sink-input-volume \"$id\" $vol >/dev/null 2>&1; "
		     "[ -n \"$mute\" ] && /usr/bin/pactl set-sink-input-mute \"$id\" \"$mute\" >/dev/null 2>&1; "
		     "done"
		     "'");
}
