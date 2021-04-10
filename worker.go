package service

import (
	"log"
	"time"
)

const minPeriodSec = 1

type worker struct {
	name     string
	period   time.Duration
	function func() error
	close    chan bool
	active   bool
}

func newWorker(name string, period time.Duration, function func() error) *worker {
	if period < minPeriodSec*time.Second {
		period = minPeriodSec * time.Second
	}

	if !validName(name) {
		log.Printf(workerErr, "", "invalid Name")
		return nil
	}

	return &worker{
		name:     name,
		period:   period,
		function: function,
		close:    make(chan bool, 1),
	}
}

func (w *worker) start() {
	log.Printf("worker: Name='%s' has been started\n", w.name)
	w.active = true
	for {
		select {
		case needClose, notEmpty := <-w.close:
			if notEmpty && needClose {
				return
			}
		default:
			if err := w.function(); err != nil {
				log.Printf(workerErr, w.name, err)
			}
		}

		time.Sleep(w.period)
	}
}

func (w *worker) stop() {
	log.Printf("worker: Name='%s' has been stopped\n", w.name)
	w.close <- true
	close(w.close)
	w.active = false
}

func (w *worker) restart() {
	log.Printf("worker: Name='%s' has been restarted\n", w.name)
	w.stop()
	w.start()
}

func (w *worker) isActive() bool {
	return w.active
}

func (w *worker) changeInterval(new time.Duration) {
	w.period = new
	if w.active {
		w.restart()
	}
}

type WorkerFunc func() error
