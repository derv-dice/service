package service

import (
	"bytes"
	"fmt"
	"github.com/jackc/pgx"
	"net/http"
	"time"
)

// Hook - структура веб хука
type Hook struct {
	name string

	client  http.Client
	service *Service // Указатель на сервис родитель для проброса коннекта к БД
}

func NewHook(name string, timeout time.Duration, service *Service) *Hook {
	return &Hook{
		name:    name,
		service: service,
		client:  http.Client{Timeout: timeout},
	}
}

func (h *Hook) Execute() (err error) {
	// Загружаем инфу о подписчиках из БД
	var s []*Subscriber
	if s, err = h.subs(); err != nil {
		return err
	}

	// Выполняем функцию
	form := registeredHookFunc.m[h.name]()
	if form == nil {
		return fmt.Errorf("hook: name='%s' error='fuction for Execute Hook not found", h.name)
	}

	// Отправка обратных запросов подписчикам
	for i := range s {
		h.sendToSubs(s[i].URL, form)
	}
	return
}

func (h *Hook) subs() (s []*Subscriber, err error) {
	s = []*Subscriber{}

	var rows *pgx.Rows
	if rows, err = h.service.DB().Query(sqlSelectSubs, h.name); err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		tmp := &Subscriber{}
		if err = rows.Scan(&tmp.URL); err != nil {
			return nil, err
		}
		s = append(s, tmp)
	}
	return
}

func (h *Hook) sendToSubs(url string, form *Form) {
	var req *http.Request
	var err error

	if req, err = http.NewRequest(http.MethodPost, url, form.Payload); err != nil {
		return
	}
	req.Header.Set("Content-Type", form.ContentType)

}

type HookFunc func() *Form

type Form struct {
	Payload     *bytes.Buffer
	ContentType string
}

type Subscriber struct {
	URL string
}
