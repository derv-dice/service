package service

import "sync"

type WorkerPool struct {
	workers map[string]*Worker
	sync.Mutex
}

func NewPorkersPool() *WorkerPool {
	return &WorkerPool{
		workers: map[string]*Worker{},
		Mutex:   sync.Mutex{},
	}
}

// Add - Добавление нового Worker в пул. Если Worker с таким именем уже есть, он останавливается и заменяется на новый
func (p *WorkerPool) Add(worker *Worker) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.stopIfActive(worker.name)

	p.workers[worker.name] = worker
}

func (p *WorkerPool) Delete(name string) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.stopIfActive(name)

	p.workers[name] = nil
}

func (p *WorkerPool) DeleteAll() {
	for name := range p.workers {
		p.Delete(name)
	}
}

func (p *WorkerPool) StopAll() {
	for name := range p.workers {
		p.stopIfActive(name)
	}
}

func (p *WorkerPool) StartAll() {
	for name := range p.workers {
		p.startIfInactive(name)
	}
}

func (p *WorkerPool) RestartAll() {
	p.StopAll()
	p.StartAll()
}

func (p *WorkerPool) stopIfActive(name string) {
	if p.workers[name] != nil && p.workers[name].IsActive() {
		p.workers[name].Stop()
	}
}

func (p *WorkerPool) startIfInactive(name string) {
	if p.workers[name] != nil && !p.workers[name].IsActive() {
		p.workers[name].Start()
	}
}
