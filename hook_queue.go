package service

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

func init() {
	go sendQueue.Run()
}

var sendQueue = NewTaskQueue()

func newSendTask(sub *Subscriber, form *Form, repeating bool) *sendTask {
	return &sendTask{sub: sub, form: form, repeating: repeating}
}

type SendTaskQueue []*sendTask

func NewTaskQueue() *SendTaskQueue {
	return &SendTaskQueue{}
}

func (q *SendTaskQueue) Run() {
	var err error
	for {
		i := q.Pop()
		if i == nil {
			continue
		}

		err = i.Execute()
		if err != nil {
			log.Printf("send queue: error='cannot send request hook_name='%s', url=%s, err='%v'", i.sub.hook.name, i.sub.URL, err)
		}
		err = nil
	}
}

func (q *SendTaskQueue) Push(i *sendTask) {
	*q = append(*q, i)
}

func (q *SendTaskQueue) Pop() (i *sendTask) {
	if len(*q) == 0 {
		return nil
	}

	h := *q
	i, *q = h[0], h[1:]
	return i
}

type sendTask struct {
	sub       *Subscriber
	form      *Form
	repeating bool
}

func (s *sendTask) Execute() (err error) {
	// Если превышен счетчик отправок у подписчика, то автоматически отписываем его (удаляем из БД)
	if s.sub.ErrCount >= maxErrCount {
		s.sub.incErrCount()
		return
	}

	var resp *http.Response
	var req *http.Request
	req, err = newRequest(s.sub, s.form)
	if err != nil {
		log.Printf(errorLog, err)
		return
	}

	resp, err = s.sub.hook.client.Do(req)
	if err != nil {
		log.Printf("hook: error sending request, url='%s' error='%v'", s.sub.URL, err)

		// Если это повторная отправка и вернулась ошибка - увеличиваем счетчик ошибок
		if s.repeating {
			s.sub.incErrCount()
			return nil
		}

		// Это значит, что хост недоступен попробуем еще раз позже
		if strings.Contains(err.Error(), "connection refused") {
			sendQueue.Push(newSendTask(s.sub, s.form, true))
		}
		return
	}

	switch resp.StatusCode / 100 {
	case 2:
		// При положительном ответе сбрасываем счетчик ошибок обратно до 0
		s.sub.resetErrCount()
	case 4:
		// Если это повторная отправка и вернулся 4xx - увеличиваем счетчик ошибок
		if s.repeating {
			s.sub.incErrCount()
			return nil
		}

		// Если вернулся 4xx код, значит хост существует, а URL указан некорректно. Можем сразу удалять такой
		_, err = s.sub.hook.service.pg.Exec(sqlDeleteSub, s.sub.hook.name, s.sub.URL)
		if err != nil {
			log.Printf(hookErr, s.sub.hook.name, fmt.Sprintf("cannot delete subscription url='%s'", s.sub.URL))
		}
		log.Printf(hookWarning, s.sub.hook.name, fmt.Sprintf("subscription url='%s' deleted cause status code 4xx received", s.sub.URL))
	case 5:
		// Если это повторная отправка и вернулся 5xx - увеличиваем счетчик ошибок
		if s.repeating {
			s.sub.incErrCount()
			return nil
		}

		// Если вернулся 5xx код, то нужно это отметить в БД и попробовать повторить отправку позже
		sendQueue.Push(newSendTask(s.sub, s.form, true))
	default:
		// Нестандартное поведение логируем
		log.Printf("hook: error sending request, url='%s' error='unexpected status code %d'", s.sub.URL, resp.StatusCode)
	}
	return
}
