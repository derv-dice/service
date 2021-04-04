package service

import (
	"log"
	"sync"
)

var registeredHookFunc = hookFuncProvider{}

type hookFuncProvider struct {
	m map[string]HookFunc
	sync.Mutex
}

func RegisterHookFunc(key string, hookFunc HookFunc) {
	registeredHookFunc.Lock()
	defer registeredHookFunc.Unlock()
	registeredHookFunc.m[key] = hookFunc
}

type HookPool struct {
	hooks map[string]*Hook
	sync.Mutex
}

func (h *HookPool) Add(hook *Hook) {
	h.Lock()
	defer h.Unlock()
	h.hooks[hook.name] = hook
}

func (h *HookPool) Trigger(name string) {
	h.Lock()
	defer h.Unlock()
	if h.hooks[name] != nil {
		if err := h.hooks[name].Execute(); err != nil {
			log.Printf("hook: name='%s' error='%v'", name, err)
		}
	} else {
		log.Printf("hook: name='%s' error='hook not exists'", name)
	}
}
