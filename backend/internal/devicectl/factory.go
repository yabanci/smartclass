package devicectl

import (
	"fmt"
	"sync"
)

type Factory struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

func NewFactory() *Factory {
	return &Factory{drivers: map[string]Driver{}}
}

func (f *Factory) Register(d Driver) {
	if d == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.drivers[d.Name()] = d
}

func (f *Factory) Get(name string) (Driver, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	d, ok := f.drivers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDriverNotFound, name)
	}
	return d, nil
}

func (f *Factory) Names() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]string, 0, len(f.drivers))
	for n := range f.drivers {
		out = append(out, n)
	}
	return out
}
