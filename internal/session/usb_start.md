# USB session start

Host side is Google Desktop Head Unit `desktop-head-unit` (SDK `extras/google/auto`, BuildID `d1ad272dfccc4fa10d76030243464ec2`), function `UsbTransport::locateGoogleDevice` in `usb_transport.cc`. Phone side is what cargadgetd does today, plus one C2 log (`~/cargadgetd.log`, 2026-08-17) where the first open died before a version request and the next open succeeded.

The emulator path never runs this. DHU without `--usb` is TCP.

## What the host does

DHU scans every USB device that has a non-empty product string. `--usb=SERIAL` restricts that to one serial. Devices already tried are skipped.

**Already an accessory.** Product id in `0x2D00`, `0x2D01`, `0x2D04`, `0x2D05` (bitmask `0x33` on `pid-0x2D00`). `0x2D02` and `0x2D03` are not treated as ready; they get the switch below.

DHU reads the active configuration. No active configuration (`LIBUSB_ERROR_NOT_FOUND`) logs `Device is in unconfigured state. Try replugging your phone.` and calls `libusb_reset_device`. Zero interfaces is a different failure (`-99`) and does not print that line.

On a configured device it looks for one interface:

| Field | Value |
|---|---|
| `bInterfaceClass` | `0xFF` |
| `bInterfaceSubClass` | `0xFF` |
| `bInterfaceProtocol` | `0x00` |
| endpoints | two bulk |

Endpoint address bit 7 set is host-read (our ep1, `0x81`). Bit 7 clear is host-write (our ep2, `0x02`). It claims that interface and starts GAL. It does **not** send AOA control requests to these product ids.

**Not an accessory yet.** Any other product id:

1. Optional `accessorystartdelaysseconds` sleep (`Delaying accessory start for %d seconds.`).
2. Vendor control transfers, timeout 0. String data includes the trailing NUL.

| bmRequestType | bRequest | wValue | wIndex | wLength | data |
|---|---|---|---|---|---|
| `0xC0` in | `51` `0x33` GET_PROTOCOL | 0 | 0 | 2 | device returns `uint16` LE protocol version |
| `0x40` out | `52` `0x34` SEND_STRING | 0 | 0 manufacturer | 8 | `Android` |
| `0x40` | `52` | 0 | 1 model | 33 | `Android Open Automotive Protocol` |
| `0x40` | `52` | 0 | 2 description | 33 | `Android Open Automotive Protocol` |
| `0x40` | `52` | 0 | 3 version | 4 | `1.0` |
| `0x40` | `52` | 0 | 4 URI | 24 | `http://www.android.com/` |
| `0x40` | `52` | 0 | 5 serial | 17 | `0000000012345678` |
| `0x40` | `58` `0x3A` SET_AUDIO_MODE | mode | 0 | 0 | only if `--aoa_audio_mode` is set |
| `0x40` | `53` `0x35` START | 0 | 0 | 0 | none |

GET_PROTOCOL must succeed (non-negative transfer). The version word is only logged (`supports AOAPv%d, starting accessory mode...`). A stall logs `does not support AOAP, skipping.`

3. `libusb_reset_device`, close, and scan again. The phone is expected to come back as one of the accessory product ids above. Outer retry sleeps 2s (`No device found ready yet, will retry shortly...`). Giving up logs `Couldn't find/access compatible USB device.`

After the interface is claimed, optional projection delay (`Delaying projection for %d seconds.`), then the host writes the first GAL frame. The phone must not speak first.

## First GAL frame

This DHU build logs `Requested protocol version: 1.7`. Pixel 9 capture (below), host → phone, one bulk write on endpoint `0x01`:

```
00 03 00 06  00 01  00 01 00 07
```

`flags 0x03` is first|last (one complete frame). Message `0x0001` is version request. Body is major `1`, minor `7`, big-endian. The Pixel answered 1.7. We answer version response `0x0002` with the same minor, status 0, still in the clear. SSL starts only after that. An older C2 log against DHU showed minor `3` (`body=00010003`); the request minor is whatever this host build asks.

## Sailfish (Jolla C2)

While the usb-moded ask dialog is up the gadget is mass storage `18d1:0afe` (Reeder / Jolla C2). The kernel accessory function already answers this host:

| Request | Result, measured 2026-09-27 |
|---|---|
| GET_PROTOCOL | `02 00` (AOAP 2). This also clears a previous start flag |
| SEND_STRING 0–5 | stored, readable back from `/dev/usb_accessory` |
| START | success, `ACCESSORY_IS_START_REQUESTED` becomes 1. idProduct stays `0x0afe` |

The kernel does not re-enumerate. `gadgetctl -aoa` (systemd `cargadgetd-aoa.service`) stops usb-moded first and links the accessory function once, then waits for that 0→1 edge with model `Android Open Automotive Protocol` or `Android Auto`. usb-moded on cable insert rewrites the gadget and drops the accessory link; relinking used to unbind every second, and a car that saw the bounce never sent START. The Desktop Head Unit retries, so it still worked. With the cable out the wait is `poll` on VBUS and does not touch `/dev/usb_accessory`. For 5 seconds after VBUS the START flag is read every 2ms, then every 250ms (a charger must not spin). A stale START still uses 2ms, because GET_PROTOCOL clears the flag and START sets it again a few milliseconds later. The moment the flag rises, the UDC is cleared before FunctionFS setup, so the host sees the disconnect then. usb-moded stays stopped until the waiter exits or the session tears the gadget down. While the waiter is running, a PC enumerates the ask gadget. The usb-moded Cargadgetd button is not installed; it used to jump straight to `2d00` and skip this.

JollaPhone2026’s ask gadget is MediaTek `0e8d:0afe` with only mass storage linked. That kernel answers GET_PROTOCOL only after `accessory.gs2` is in the configuration (`android_acc_setup_composite`; the C2’s `android_setup` answers without the link). `gadgetctl -aoa` links `accessory.gs2` before waiting when `idVendor` is not `0x18d1`, and writes `bMaxPacketSize0` `0x40` if it is 0. That link is removed before `18d1:2d00` is bound: the accessory function adds an interrupt IN beside the bulk pair, and a car that still sees it configures USB without sending the version request on FunctionFS. The bound config is only `ffs.aa`. DHU does not filter on vendor `18d1`; a stalled GET_PROTOCOL is what skips the phone.

## What we do instead

`gadgetctl` binds `idVendor=0x18d1` `idProduct=0x2d00` and leaves the phone's manufacturer, product, and serial strings as they were (a Pixel stays `Google` / `Pixel 9` / its real serial). One vendor interface `FF/FF/00`, bulk `0x81` and `0x02`. That is the “already accessory” shape, so DHU skips steps 1–3 and goes straight to the version request. `GET_PROTOCOL` on this gadget is answered with AOAP version 2; any other SETUP is stalled.

`cargadgetd-aoa.service` is what stops usb-moded and starts the MD, after START.

FunctionFS SETUP events are stalled. An IN request (GET_PROTOCOL, `0xC0`) stalls on a read of ep0; an OUT request stalls on a write. That is what Linux `f_fs.c` treats as `usb_ep_set_halt`. Leaving the request unanswered holds the host until its control timeout. DHU does not send GET_PROTOCOL when the product id is already `0x2D00`. A head unit that repeats it after the switch gets the stall. The stall runs in the MD's ep0 reader, so it is in effect only after that binary is installed.

## Why the first plug dies

C2, 2026-08-17, against this DHU:

```
handshake: waiting for version request
handshake: read failed … frames=0 … transport endpoint shutdown
… reopen …
handshake: first frame … msg=0x0001 after 3.7s
version request 1.3
```

That shutdown is the host bus-reset. DHU resets when `libusb_get_active_config_descriptor` fails, which is the window while the gadget is bound but not configured: UDC bind, FunctionFS descriptors, or the usb-moded switch tearing the previous gadget down. The MD’s first `ep2` read is in that window, the reset kills it, DHU sleeps 2s and scans again, and the second open gets the version request.

Pressing **Mode cargadgetd** during that scan is the bad moment. The button unbinds whatever usb-moded had enumerated and publishes `18d1:2d00`. A host already in `locateGoogleDevice` sees an unconfigured device, resets the bus, and the new MD session dies with zero frames. A press while nothing is scanning, or DHU’s own retry after the reset, is the one that sticks. Replug is the same retry.

A head unit that only starts Android Auto by sending the table above will not see those requests answered. They land on the usb-moded charging/MTP gadget (stalled) if the button has not been pressed, or on our accessory gadget (logged, not completed) if it has. We never re-enumerate in response to START, because we are already `0x2D00`.

## Measured, Pixel 9, 2026-09-27

usbmon on this laptop, then DHU `--usb`. The cable was already plugged; the phone was idle MTP. That idle state is not the handshake. The trace starts when DHU runs.

Before the switch: `18d1:4ee1`, product `Pixel 9`, serial `46170DLAQ0020F`, one PTP interface (class 6). About 1 second of control transfers:

| Time | Request | Payload |
|---|---|---|
| +0 ms | `C0 33` GET_PROTOCOL, wLength 2 | phone is AOAP version **2** (DHU: `supports AOAPv2`) |
| +0.2 ms | `40 34` index 0 | `Android` |
| | `40 34` index 1 | `Android Open Automotive Protocol` |
| | `40 34` index 2 | `Android Open Automotive Protocol` |
| | `40 34` index 3 | `1.0` |
| | `40 34` index 4 | `http://www.android.com/` |
| | `40 34` index 5 | `0000000012345678` |
| +1.4 ms | `40 35` START | no data |

Then a bus reset. The phone came back as device 3: `18d1:2d00`, manufacturer `Google`, product `Pixel 9`, same serial, USB 3.2, one interface `FF/FF/00`, bulk `0x81` IN and `0x01` OUT, 1024-byte packets. `SET_CONFIGURATION` 1 followed.

DHU’s own retry could not open that node (`/dev/bus/usb/008/003` was `root:root` `0664`, not `plugdev`) and exited with `Couldn't find/access compatible USB device` while the phone was already `2d00`. A second DHU launch after `chmod a+rw` on that node logged `Found accessory: ifnum: 0, rd_ep: 129, wr_ep: 1`, `Phone reported protocol version 1.7`, and `SSL negotiation finished successfully` (`TLSv1.2`, `ECDHE-RSA-AES128-GCM-SHA256`). First bulk write is the version request above.

A car that sends different strings would still be request `52` with the same indexes; only those payload bytes would differ. No car usbmon yet.
