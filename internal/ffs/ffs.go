// Package ffs writes FunctionFS descriptors for the AOA accessory gadget.
package ffs

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// Constants from linux/usb/functionfs.h and ch9.h (uapi enum values, not legacy hex).
const (
	functionfsDescriptorsMagicV2 = 3
	functionfsStringsMagic       = 2
	functionfsHasFSDesc          = 1
	functionfsHasHSDesc          = 2

	usbDTInterface        = 0x04
	usbDTEndpoint         = 0x05
	usbDirIn              = 0x80
	usbDirOut             = 0x00
	usbEndpointXferBulk   = 0x02
	usbClassVendorSpec    = 0xff
	usbSubclassVendorSpec = 0xff
)

// SetupAccessory writes descriptors + strings to ep0.
// The returned ep0 file MUST stay open for the lifetime of the gadget bind —
// closing it resets FunctionFS (EBUSY / dead UDC).
func SetupAccessory(ffsDir string) (*os.File, error) {
	ep0 := filepath.Join(ffsDir, "ep0")
	f, err := os.OpenFile(ep0, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open ep0: %w", err)
	}

	desc := buildAccessoryDescriptors()
	if _, err := f.Write(desc); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("write descriptors: %w", err)
	}
	strs := buildStrings()
	if _, err := f.Write(strs); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("write strings: %w", err)
	}
	return f, nil
}

func buildAccessoryDescriptors() []byte {
	iface := func() []byte {
		return []byte{
			9, usbDTInterface, 0, 0, 2,
			usbClassVendorSpec, usbSubclassVendorSpec, 0x00, 1,
		}
	}
	ep := func(addr byte, maxPacket uint16) []byte {
		b := []byte{7, usbDTEndpoint, addr, usbEndpointXferBulk, 0, 0, 0}
		binary.LittleEndian.PutUint16(b[4:6], maxPacket)
		return b
	}
	fs := append(iface(), ep(1|usbDirIn, 64)...)
	fs = append(fs, ep(2|usbDirOut, 64)...)
	hs := append(iface(), ep(1|usbDirIn, 512)...)
	hs = append(hs, ep(2|usbDirOut, 512)...)

	body := make([]byte, 0, 128)
	body = append(body, make([]byte, 12)...)
	body = binary.LittleEndian.AppendUint32(body, 3)
	body = binary.LittleEndian.AppendUint32(body, 3)
	body = append(body, fs...)
	body = append(body, hs...)
	binary.LittleEndian.PutUint32(body[0:4], functionfsDescriptorsMagicV2)
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(body)))
	binary.LittleEndian.PutUint32(body[8:12], functionfsHasFSDesc|functionfsHasHSDesc)
	return body
}

func buildStrings() []byte {
	str := []byte("Android Accessory Interface")
	inner := make([]byte, 0, 64)
	inner = binary.LittleEndian.AppendUint16(inner, 0x0409)
	inner = append(inner, str...)
	inner = append(inner, 0)

	body := make([]byte, 16, 64)
	binary.LittleEndian.PutUint32(body[0:4], functionfsStringsMagic)
	binary.LittleEndian.PutUint32(body[8:12], 1)
	binary.LittleEndian.PutUint32(body[12:16], 1)
	body = append(body, inner...)
	binary.LittleEndian.PutUint32(body[4:8], uint32(len(body)))
	return body
}
