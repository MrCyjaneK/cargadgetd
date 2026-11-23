package control

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/cargadgetd/cargadgetd/internal/livevid"
)

var benchRun = func(encoder string, status func(string), live func(livevid.Cell), cell func(livevid.Cell)) livevid.Result {
	return livevid.Run(livevid.BenchOptions{
		Encoder:  encoder,
		OnStatus: status,
		OnLive:   live,
		OnCell:   cell,
	})
}

type benchView struct {
	Encoder   string         `json:"encoder"`
	CanSelect bool           `json:"canSelect"`
	Live      *livevid.Cell  `json:"live,omitempty"`
	Rows      []livevid.Cell `json:"rows"`
}

var (
	benchMu      sync.Mutex
	benchRunning bool
	benchDiscard bool
	benchGen     int
	benchStatus  string
	benchRaw     []byte
)

func benchJSON(encoder string, cells []livevid.Cell, live *livevid.Cell) []byte {
	if cells == nil {
		cells = []livevid.Cell{}
	}
	raw, err := json.Marshal(benchView{
		Encoder:   encoder,
		CanSelect: livevid.CanSelect(cells),
		Live:      live,
		Rows:      cells,
	})
	if err != nil {
		return []byte(`{"encoder":"","canSelect":false,"rows":[]}`)
	}
	return raw
}

func BenchStart(encoder string) string {
	encoder = strings.TrimSpace(encoder)
	if encoder == "" {
		return "encoder required"
	}
	benchMu.Lock()
	if benchRunning {
		benchMu.Unlock()
		return "benchmark already running"
	}
	benchGen++
	gen := benchGen
	benchDiscard = false
	benchRunning = true
	benchStatus = "Starting"
	benchRaw = benchJSON(encoder, nil, nil)
	benchMu.Unlock()

	go func() {
		var cells []livevid.Cell
		var live *livevid.Cell
		res := benchRun(encoder, func(s string) {
			benchMu.Lock()
			if benchGen == gen && !benchDiscard {
				benchStatus = s
			}
			benchMu.Unlock()
		}, func(c livevid.Cell) {
			benchMu.Lock()
			if benchGen == gen && !benchDiscard {
				cp := c
				live = &cp
				benchRaw = benchJSON(encoder, cells, live)
			}
			benchMu.Unlock()
		}, func(c livevid.Cell) {
			benchMu.Lock()
			if benchGen == gen && !benchDiscard {
				cells = append(cells, c)
				live = nil
				benchRaw = benchJSON(encoder, cells, nil)
			}
			benchMu.Unlock()
		})
		benchMu.Lock()
		defer benchMu.Unlock()
		benchRunning = false
		if benchGen != gen || benchDiscard {
			benchDiscard = false
			benchStatus = ""
			benchRaw = nil
			return
		}
		benchStatus = ""
		name := res.Encoder
		if name == "" {
			name = encoder
		}
		rows := res.Cells
		if rows == nil {
			rows = cells
		}
		benchRaw = benchJSON(name, rows, nil)
	}()
	return ""
}

func BenchRunning() bool {
	benchMu.Lock()
	defer benchMu.Unlock()
	return benchRunning
}

func BenchStatus() string {
	benchMu.Lock()
	defer benchMu.Unlock()
	return benchStatus
}

func BenchResult() string {
	benchMu.Lock()
	defer benchMu.Unlock()
	if len(benchRaw) == 0 {
		return `{"encoder":"","canSelect":false,"rows":[]}`
	}
	return string(benchRaw)
}

func BenchWatch() {
	benchMu.Lock()
	benchDiscard = false
	benchMu.Unlock()
}

func BenchClear() {
	benchMu.Lock()
	defer benchMu.Unlock()
	if benchRunning {
		benchDiscard = true
		benchStatus = ""
		benchRaw = nil
		return
	}
	benchStatus = ""
	benchRaw = nil
}
