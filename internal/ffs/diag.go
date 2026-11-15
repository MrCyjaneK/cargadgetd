package ffs

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cargadgetd/cargadgetd/internal/cfg"
)

// Optional wire log for correlating FunctionFS writes with DHU BAD_DECRYPT.
// Enable with usb.ffs_diag: /path/to/ffs.diag (TSV lines).

var (
	diagMu   sync.Mutex
	diagFile *os.File
	writeSeq atomic.Uint64
	readSeq  atomic.Uint64
)

func diagPath() string { return strings.TrimSpace(cfg.Get().USB.FFSDiag) }

func diagEnabled() bool { return diagPath() != "" }

func diagLog(format string, args ...any) {
	if !diagEnabled() {
		return
	}
	diagMu.Lock()
	defer diagMu.Unlock()
	if diagFile == nil {
		f, err := os.OpenFile(diagPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		diagFile = f
		fmt.Fprintf(diagFile, "# t_ns\tseq\tevent\tn\tmod512\tms\terr\taa_ch\taa_flags\taa_plen\n")
	}
	fmt.Fprintf(diagFile, "%d\t"+format+"\n", append([]any{time.Now().UnixNano()}, args...)...)
}

// peekAA parses the cleartext AA header at the start of a USB write (if present).
func peekAA(p []byte) (ch, flags uint8, plen int) {
	if len(p) < 4 {
		return 0, 0, -1
	}
	ch, flags = p[0], p[1]
	plen = int(binary.BigEndian.Uint16(p[2:4]))
	return ch, flags, plen
}
