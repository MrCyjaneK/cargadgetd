//go:build !linux

package ffs

type aioContext struct{}

func (e *Endpoints) initAIO()  {}
func (e *Endpoints) closeAIO() {}

func (e *Endpoints) writeAIO(p []byte, yield func()) (int, error) {
	_ = yield
	return e.writeSync(p)
}
