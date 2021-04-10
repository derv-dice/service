package service

import "sync"

type workerPool struct {
	parent  *Service
	workers map[string]*worker
	sync.Mutex
}

func newWorkerPool(parent *Service) *workerPool {
	return &workerPool{
		parent:  parent,
		workers: map[string]*worker{},
		Mutex:   sync.Mutex{},
	}
}

func (p *workerPool) add(worker *worker) {
	p.Lock()
	defer p.Unlock()
	p.stopIfActive(worker.name)
	p.workers[worker.name] = worker
}

func (p *workerPool) delete(name string) {
	p.Lock()
	defer p.Unlock()
	p.stopIfActive(name)
	delete(p.workers, name)
}

func (p *workerPool) deleteAll() {
	for name := range p.workers {
		p.delete(name)
	}
}

func (p *workerPool) stopAll() {
	for name := range p.workers {
		go p.stopIfActive(name)
	}
}

func (p *workerPool) startAll() {
	for name := range p.workers {
		go p.startIfInactive(name)
	}
}

func (p *workerPool) restartAll() {
	p.stopAll()
	p.startAll()
}

func (p *workerPool) startByName(name string) { go p.startIfInactive(name) }
func (p *workerPool) stopByName(name string)  { go p.stopIfActive(name) }

func (p *workerPool) stopIfActive(name string) {
	if p.workers[name] != nil && p.workers[name].isActive() {
		p.workers[name].stop()
	}
}

func (p *workerPool) startIfInactive(name string) {
	if p.workers[name] != nil && !p.workers[name].isActive() {
		p.workers[name].start()
	}
}
