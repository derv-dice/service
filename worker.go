package service

import (
	"log"
	"time"
)

// Worker - Функция, выполняющаяся в фоне с заданным интервалом
type Worker struct {
	name     string
	interval time.Duration
	f        func() error
	closer   chan bool
	active   bool
}

func NewWorker(name string, interval time.Duration, Func func() error) *Worker {
	return &Worker{
		name:     name,
		interval: interval,
		f:        Func,
		closer:   make(chan bool, 1),
	}
}

func (w *Worker) Start() {
	log.Printf("worker: name='%s' has been started\n", w.name)
	w.active = true
	for {
		select {
		case needClose, notEmpty := <-w.closer:
			if notEmpty && needClose {
				return
			}
		default:
			if err := w.f(); err != nil {
				log.Printf("worker: name='%s' error='%v'", w.name, err)
			}
		}

		time.Sleep(w.interval)
	}
}

func (w *Worker) Stop() {
	log.Printf("worker: name='%s' has been stopped\n", w.name)
	w.closer <- true
	close(w.closer)
	w.active = false
}

func (w *Worker) Restart() {
	log.Printf("worker: name='%s' has been restarted\n", w.name)
	w.Stop()
	w.Start()
}

func (w *Worker) IsActive() bool {
	return w.active
}

func (w *Worker) ChangeInterval(new time.Duration) {
	w.interval = new
	if w.active {
		w.Restart()
	}
}