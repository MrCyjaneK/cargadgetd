package ffs

import "testing"

func TestUSBReadBufFitsMTU3(t *testing.T) {
	if usbReadBuf%512 != 0 {
		t.Fatalf("usbReadBuf %d is not a multiple of 512", usbReadBuf)
	}
	// mtu3_gadget_queue rejects req->length > GPD_BUF_SIZE (65532) with
	// EOPNOTSUPP before it considers the gen2cp 1048572 limit.
	if usbReadBuf > 65532 {
		t.Fatalf("usbReadBuf %d exceeds mtu3 GPD_BUF_SIZE", usbReadBuf)
	}
}
