# HU → MD events (DHU USB capture, 2026-08-16)

Gate: `DEVICE=192.168.1.177 SOAK_SECS=90 GAL_CAPTURE=1 ./scripts/test-device.sh`

DHU 2.0-linux (`2022-03-30-438482292`) `--usb` with `/tmp/dhu-gal-capture.ini` (touch + controller, sensors on, 800×480). After `video_tx`, stdin from `scripts/test-device.sh` (`/tmp/gal-inject-sent.txt`). Phone: `apps.unhandled_log` → `/tmp/cargadgetd-unhandled.log`.

Both soaks healthy (`tx2=1041` then `1415`, `decode_err=0`, `ping_to=0`). No `UNHANDLED`. No `SENSOR` / `ButtonsEvent`.

## What arrived on the wire

| Capture | GAL | Corresponds to |
|---|---|---|
| `SDR sensor ch=1 types=1:Location,2:Compass,3:CarSpeed,5:Odometer,6:FuelLevel,7:ParkingBrake,8:Gear,10:NightData,13:DrivingStatus,19:Accel,20:Gyro` | ServiceDiscovery sensor channel | DHU `[sensors]` ini - **advertise only** |
| `SDR input ch=3 touch=800x480 buttons=[1…23, 81, 84, 85, 87, 88, 126, 127, 268–271, 65536, 65537, 65538, 65540]` | Input channel, 800×480. Buttons are **packed** varints in field 1 (not repeated `08 <code>`) | `touch=true` `controller=true` `resolution=800x480` |
| `SDR channel=6 kind=vendor` | Vendor extension | DHU example `EchoVendorExtension` / `com.example.echovendor`. Ignored |
| `SDR channel=7 kind=media_input` | Mic uplink 16 kHz s16le mono | DHU default mic. Not opened |
| `VIDEO_FOCUS focus=1 body=08011001` | VideoFocusIndication field1=1 (focused), field2=1 | Session start |
| `INPUT_HANDSHAKE buttons=[… packed extras + 86 …] hu_sdr=[packed list]` | MD handshake echoes built-in set **plus** HU SDR extras | HU only sends keys listed here |
| `INPUT touch=400,240 action=0` then `action=1` | `InputEvent` 0x8001, `TouchEvent` **field 3**. action 0=Press, 1=Release. Pixels | DHU **`tap 400 240`** - only stdin command that produced GAL |

Touch body (tap): timestamp field 1, `TouchEvent` `0x1a`: location x=400 y=240 pointer=0, action_index=0, action=0/1. Matches `proto/aa.proto`. **ButtonsEvent field 4 vs 5 is still unverified** (no button frame arrived).

## DHU `help` (this binary)

Categories below are verbatim from DHU stdout after stdin `help`. **Bold** = we injected it; **wire** = it showed up in `unhandled_log`.

### Controller (`dpad …`)

Expected: `InputEvent` scan_code (or relative 65536+). **None arrived.**

| Command | Expected code |
|---|---|
| `dpad up/down/left/right` | 19–22 |
| `dpad ur/dl/ul/dr` | 270 / 269 / 268 / 271 (`KEYCODE_DPAD_*`) |
| `dpad click` | 23 |
| `dpad back` | 4 |
| `dpad soft left/right` | 1 / 2 |
| `dpad 0`–`9` `*` `#` `+` | 7–16, 17, 18, 81 |
| `dpad rotate left/right` | 65536 rotary (unverified which side) |
| `dpad flick left/right` | relative / same rotary family |

### Keycode (`keycode <name>`)

DHU names are **not** numeric scan codes. `keycode 3` is the **digit 3** key (code 10), not HOME. `keycode 87` is not a command.

Expected: `InputEvent` `ButtonsEvent` scan_code. **None arrived** (named cmds were accepted; no `[E]`). Same input channel as `tap`, so this looks like an SDL key path that never became GAL (window unfocused), not a missing handshake.

| Command | Scan code | AACS / Android |
|---|---|---|
| `keycode soft_left` / `soft_right` | 1 / 2 | MICROPHONE_2 / MENU |
| `keycode home` | 3 | HOME |
| `keycode back` | 4 | BACK |
| `keycode call` / `endcall` | 5 / 6 | PHONE / CALL_END |
| `keycode 0` … `9` | 7–16 | KEYCODE_0…9 |
| `keycode star` / `pound` / `plus` | 17 / 18 / 81 | `*` `#` `+` |
| `keycode dpad_up` … `dpad_center` | 19–23 | DPAD |
| `keycode search` | 84 | MICROPHONE_1 / SEARCH |
| `keycode media_play_pause` | 85 | TOGGLE_PLAY |
| `keycode media_next` / `media_previous` | 87 / 88 | NEXT / PREV |
| `keycode media_play` / `media_pause` | 126 / 127 | PLAY / PAUSE |
| `keycode dpad_up_left` … `down_right` | 268–271 | DPAD diagonals |
| `keycode rotary` | 65536 | SCROLL_WHEEL |
| `keycode media` / `navigation` / `tel` | 65537 / 65538 / 65540 | DHU packed extras after rotary (help order) |

Our handshake also claims **86 MEDIA_STOP** (not in DHU SDR).

### Day/Night

| Command | Expected | Wire |
|---|---|---|
| `night` `day` `daynight` `nightday` | `SensorEvent` type 10 NightData (inner field 1: 0=day, 1=night) | **none** - DHU accepted; no StartRequest |

### Driving restrictions

| Command | Expected | Wire |
|---|---|---|
| `restrict none` / `restrict all` | `SensorEvent` type 13 DrivingStatus (0 = UNRESTRICTED; bits 1 NO_VIDEO, 2 NO_KEYBOARD, 4 NO_VOICE, 8 NO_CONFIG, 16 LIMIT_MESSAGE) | **none** |

Help only lists `none` and `all`. Binary strings also have `restrict video|keyboard|voice|config|message`.

### Fuel / misc sensors

All accepted; **no `SensorEvent`** (MD does not send `SensorStartRequest` - DHU `gal.SensorRequest` required-fields fail → disconnect).

| Command | Type | Notes |
|---|---|---|
| `location lat long [acc alt speed bearing]` | 1 Location | |
| `compass bearing [pitch] [roll]` | 2 Compass | |
| `speed m/s` | 3 CarSpeed | |
| `odometer kms [trip_kms]` | 5 Odometer | |
| `fuel [percent]` | 6 FuelLevel | `range` / `lowfuel` also in help |
| `accel x y z` | 19 Accel | m/s² |
| `gyro x y z` | 20 Gyro | rad/s |

**Not in DHU 2.0-linux CLI at all:** `parking_brake`, `gear` (SDR still advertises types 7 and 8). `help parking_brake` / `help gear` → command not found.

Not in this DHU SDR: RPM(4), Diagnostics(9), Environment(11), HVAC(12), DeadReckoning(14), Passenger(15), Door(16), Light(17), Tire(18), GPS(21).

### Other DHU commands (not injected)

| Command | Likely GAL |
|---|---|
| `focus audio` / `focus nav` / `focus video` / `focus video toggle` | AudioFocus / NavFocus / VideoFocusIndication |
| `mic begin` / `play` / `repeat` / `reject` | Voice / mic input |
| `tollcard insert` / `remove` | Sensor (not in our type table) |
| `tap x y` | **Touch InputEvent - verified** |
| `help` `quit` `exit` `licenses` `sleep` `screenshot` | Local DHU |

## Why sensors/keys stay off the wire

1. **Sensors:** HU streams `SensorEvent` after MD `SensorStartRequest`. We open+ACK only. DHU stdin (`night`, `speed`, …) only updates DHU’s local source.
2. **Keys:** Handshake listed every DHU packed code (second soak). `tap` still worked; `keycode`/`dpad` still produced zero `INPUT`. Car steering keys are dispatched to every plugin as `cargadgetd_app_<id>_huinput_<event>(pressed)`. Unknown scan codes stay in `apps.unhandled_log` as `INPUT unknown_button`.

## Re-run

```bash
DEVICE=192.168.1.177 SOAK_SECS=90 GAL_CAPTURE=1 ./scripts/test-device.sh
# capture: /tmp/cargadgetd-unhandled.log
# inject:  /tmp/gal-inject-sent.txt
# DHU:     /tmp/dhu-device-test.log   (includes `help` dump)
```
