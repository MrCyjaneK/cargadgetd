package session

import (
	"errors"
	"io"
	"testing"
)

type stubBusy struct{}

func (stubBusy) Error() string { return "busy" }
func (stubBusy) USBWriteBusy() {}

func TestWriteBusyDoesNotNeedFFS(t *testing.T) {
	if !isUSBWriteBusy(stubBusy{}) {
		t.Fatal("busy")
	}
	if isUSBWriteBusy(io.EOF) {
		t.Fatal("eof")
	}
	if !isUSBWriteBusy(errors.Join(io.ErrClosedPipe, stubBusy{})) {
		t.Fatal("wrapped")
	}
}
