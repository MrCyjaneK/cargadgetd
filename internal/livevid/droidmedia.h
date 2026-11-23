#ifndef CARGADGETD_DROIDMEDIA_H
#define CARGADGETD_DROIDMEDIA_H

#include <stdint.h>
#include <stdbool.h>
#include <sys/types.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct {
	void *data;
	ssize_t size;
} DroidMediaData;

typedef void (*DroidMediaCallback)(void *data);

typedef struct {
	DroidMediaCallback ref;
	DroidMediaCallback unref;
	void *data;
} DroidMediaBufferCallbacks;

typedef enum {
	DROID_MEDIA_CODEC_SW_ONLY = 0x1,
	DROID_MEDIA_CODEC_HW_ONLY = 0x2,
	DROID_MEDIA_CODEC_USE_EXTERNAL_LOOP = 0x4,
	DROID_MEDIA_CODEC_NO_MEDIA_BUFFER = 0x8,
} DroidMediaCodecFlags;

typedef enum {
	DROID_MEDIA_CODEC_BITRATE_CONTROL_DEFAULT = 0,
	DROID_MEDIA_CODEC_BITRATE_CONTROL_VBR = 1,
	DROID_MEDIA_CODEC_BITRATE_CONTROL_CBR = 2,
} DroidMediaCodecBitrateMode;

typedef struct {
	const char *type;
	int32_t width;
	int32_t height;
	int32_t fps;
	int32_t channels;
	int32_t sample_rate;
	int32_t hal_format;
	DroidMediaCodecFlags flags;
} DroidMediaCodecMetaData;

typedef struct {
	int32_t profile;
	int32_t level;
	int32_t prepend_header_to_sync_frames;
} DroidMediaCodecEncoderH264Settings;

typedef struct {
	DroidMediaCodecMetaData parent;
	int32_t color_format;
	int32_t bitrate;
	int32_t meta_data;
	int32_t stride;
	int32_t slice_height;
	int32_t max_input_size;
	int32_t bitrate_mode;
	union {
		DroidMediaCodecEncoderH264Settings h264;
	} codec_specific;
} DroidMediaCodecEncoderMetaData;

/* droidmedia 0.20260819 appended stride and slice_height to the parent.
   That shifts every field the encoder reads. Hardware (alternative) fills
   this layout. The struct above is the older HAL and stays as it was. */
typedef struct {
	const char *type;
	int32_t width;
	int32_t height;
	int32_t fps;
	int32_t channels;
	int32_t sample_rate;
	int32_t hal_format;
	DroidMediaCodecFlags flags;
	int32_t stride;
	int32_t slice_height;
} DroidMediaCodecMetaDataAlt;

typedef struct {
	DroidMediaCodecMetaDataAlt parent;
	int32_t color_format;
	int32_t bitrate;
	int32_t meta_data;
	int32_t stride;
	int32_t slice_height;
	int32_t max_input_size;
	int32_t bitrate_mode;
	union {
		DroidMediaCodecEncoderH264Settings h264;
	} codec_specific;
} DroidMediaCodecEncoderMetaDataAlt;

typedef struct {
	DroidMediaData data;
	int64_t ts;
	int64_t decoding_ts;
	bool sync;
	bool codec_config;
} DroidMediaCodecData;

typedef struct {
	void (*signal_eos)(void *data);
	void (*error)(void *data, int err);
	int (*size_changed)(void *data, int32_t width, int32_t height);
} DroidMediaCodecCallbacks;

typedef struct {
	void (*data_available)(void *data, DroidMediaCodecData *encoded);
} DroidMediaCodecDataCallbacks;

typedef struct _DroidMediaCodec DroidMediaCodec;

#ifdef __cplusplus
}
#endif
#endif
