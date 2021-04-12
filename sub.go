package service

import (
	"fmt"
	"log"
)

type Subscriber struct {
	*hook
	URL      string
	Pass     string
	ErrCount int
}

func (s *Subscriber) Subscribe() (passCode string, err error) {

	return
}

func (s *Subscriber) Unsubscribe(passCode string) (err error) {

	return
}

func (s *Subscriber) resetErrCount() {
	if s.hook == nil {
		log.Printf(hookErr, "", "invalid nil pointer reference")
		return
	}

	_, err := s.hook.service.pg.Exec(sqlResetSubErrCount, s.hook.name, s.URL)
	if err != nil {
		log.Printf(hookErr, s.hook.name, fmt.Sprintf("cannot reset err_count for url='%s'", s.URL))
	}
}

func (s *Subscriber) incErrCount() {
	if s.hook == nil {
		log.Printf(hookErr, "", "invalid nil pointer reference")
		return
	}

	// Если предел ошибок превышен, то удаляем подписку
	if s.ErrCount >= maxErrCount {
		_, err := s.hook.service.pg.Exec(sqlDeleteSub, s.hook.name, s.URL)
		if err != nil {
			log.Printf(hookErr, s.hook.name, fmt.Sprintf("cannot delete subscription hook_name='%s', url='%s'", s.hook.name, s.URL))
		}
		log.Printf(hookWarning, s.hook.name, fmt.Sprintf("subscription hook_name='%s', url='%s' deleted cause error limit exceeded", s.hook.name, s.URL))
		return
	}

	_, err := s.hook.service.pg.Exec(sqlIncrementSubErrCount, s.hook.name, s.URL)
	if err != nil {
		log.Printf(hookErr, s.hook.name, fmt.Sprintf("cannot increment err_count for subscription hook_name='%s', url='%s'", s.hook.name, s.URL))
		return
	}
	log.Printf(hookWarning, s.hook.name, fmt.Sprintf("subscription hook_name='%s', url='%s' err_count incremented", s.hook.name, s.URL))
}
