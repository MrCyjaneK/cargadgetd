package livevid

import "time"

type Phase struct {
	YUV    time.Duration
	Submit time.Duration
	Take   time.Duration
}

var OnPhase func(Phase)
