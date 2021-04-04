package service

import (
	"log"
	"sync"
)

var registeredHookFunc = hookFuncProvider{}

type hookFuncProvider struct {
	m map[string]*HookFunc
	sync.Mutex
}

func RegisterHookFunc(key string, hookFunc *HookFunc) {
	registeredHookFunc.Lock()
	defer registeredHookFunc.Unlock()
	registeredHookFunc.m[key] = hookFunc
}

type HookPool struct {
	hooks map[string]*Hook
	sync.Mutex
}

// Trigger -
func (h *HookPool) Trigger(name string) {
	h.Lock()
	defer h.Unlock()
	if h.hooks[name] != nil {
		h.hooks[name].execute()
	} else {
		log.Printf("Hook: name='%s' error='Hook not exists'", name)
	}
}
