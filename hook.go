package service

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"
)

// Hook - структура веб хука
type Hook struct {
	name string

	http.Client
	*Service // Указатель на сервис родитель для проброса коннекта к БД
}

func NewHook(name string, timeout time.Duration, service *Service) *Hook {
	return &Hook{
		name:    name,
		Service: service,
		Client:  http.Client{Timeout: timeout},
	}
}

func (h *Hook) execute() (err error) {
	// Загружаем инфу о подписчиках из БД
	//TODO subs()

	// Выполняем функцию
	form := registeredHookFunc.m[h.name]
	if form == nil {
		return fmt.Errorf("hook: name='%s' error='fuction for execute Hook not found", h.name)
	}

	// Отправка обратных запросов подписчикам

	return
}

func subs() {

}

func (h *Hook) sendToSubs(url string, form *Form) {
	var req *http.Request
	var err error

	if req, err = http.NewRequest(http.MethodPost, url, form.Payload); err != nil {
		return
	}
	req.Header.Set("Content-Type", form.ContentType)

}

type HookFunc func() *multipart.Form

type Form struct {
	Payload     *bytes.Buffer
	ContentType string
}

type Subscriber struct {
	URL string
}
