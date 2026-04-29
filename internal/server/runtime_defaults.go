package server

import (
	"sync"

	"github.com/ponchione/sodoryard/internal/config"
)

// RuntimeDefaults stores the effective default provider/model for live runtime
// surfaces like /api/config and /api/ws. It starts from config defaults and can
// be updated at runtime without mutating the loaded config struct.
type RuntimeDefaults struct {
	mu       sync.RWMutex
	provider string
	model    string
}

const (
	forcedRuntimeDefaultProvider = "codex"
	forcedRuntimeDefaultModel    = "gpt-5.5"
)

func lockedRuntimeDefault() (provider string, model string) {
	return forcedRuntimeDefaultProvider, forcedRuntimeDefaultModel
}

func runtimeDefaultOverrideAllowed(provider string, model string) bool {
	lockedProvider, lockedModel := lockedRuntimeDefault()
	return provider == lockedProvider && model == lockedModel
}

func NewRuntimeDefaults(_ *config.Config) *RuntimeDefaults {
	rd := &RuntimeDefaults{}
	provider, model := lockedRuntimeDefault()
	rd.provider = provider
	rd.model = model
	return rd
}

func (r *RuntimeDefaults) Get() (provider string, model string) {
	if r == nil {
		return "", ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.provider, r.model
}

func (r *RuntimeDefaults) Set(provider string, model string) {
	if r == nil {
		return
	}
	provider, model = lockedRuntimeDefault()
	r.mu.Lock()
	defer r.mu.Unlock()
	if provider != "" {
		r.provider = provider
	}
	if model != "" {
		r.model = model
	}
}
