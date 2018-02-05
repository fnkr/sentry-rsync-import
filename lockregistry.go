package main

import (
	"sync"
)

type LockRegistry struct {
	lock  sync.Mutex
	locks map[string]bool
}

func (reg *LockRegistry) Init() {
	reg.locks = make(map[string]bool)
}

func (reg *LockRegistry) TryLock(key string) bool {
	reg.lock.Lock()
	defer reg.lock.Unlock()
	if _, ok := reg.locks[key]; ok {
		return false
	}
	reg.locks[key] = true
	return true
}

func (reg *LockRegistry) Unlock(key string) {
	reg.lock.Lock()
	defer reg.lock.Unlock()
	delete(reg.locks, "key")
}
