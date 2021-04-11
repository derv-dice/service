package service

import (
	"bytes"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx"
)

const maxErrCount = 3
const httpClientTimeoutSec = 10

// hook - структура веб хука
type hook struct {
	name     string
	client   http.Client
	service  *Service // Указатель на сервис родитель для проброса коннекта к БД
	function HookFunc
}

func newHook(name string, function HookFunc, parent *Service) *hook {
	if parent == nil {
		return nil
	}

	if !validName(name) {
		log.Printf(hookErr, "", "invalid name'")
		return nil
	}

	return &hook{
		name:     name,
		service:  parent,
		client:   http.Client{Timeout: time.Second * httpClientTimeoutSec},
		function: function,
	}
}

func (h *hook) trigger() (err error) {
	// Загружаем инфу о подписчиках из БД
	var s []*Subscriber
	if s, err = h.loadSubs(); err != nil {
		return err
	}

	// Если подписчиков нет, то и делать ничего не нужно
	if len(s) == 0 {
		return
	}

	// Выполняем функцию
	form := h.function.Function()
	if form == nil {
		return fmt.Errorf("hook: name='%s' error='fuction for hook.trigger() not found", h.name)
	}

	// Отправка обратных запросов подписчикам
	for i := range s {
		// TODO ограничить максимальное количество одновременно работающих горутин, введя очередь отправки запросов
		//	 p.s. Пока что не нужно
		go h.sendToSub(s[i], form)
	}
	return
}

func (h *hook) loadSubs() (s []*Subscriber, err error) {
	s = []*Subscriber{}

	var rows *pgx.Rows
	if rows, err = h.service.pg.Query(sqlSelectSubs, h.name); err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		tmp := &Subscriber{hook: h}
		if err = rows.Scan(&tmp.URL, &tmp.Pass, &tmp.ErrCount); err != nil {
			return nil, err
		}
		s = append(s, tmp)
	}
	return
}

func newRequest(sub *Subscriber, form *Form) (req *http.Request, err error) {
	data, contentType, err := form.Data()
	if err != nil {
		return
	}

	switch form {
	case nil:
		req, _ = http.NewRequest(http.MethodPost, sub.URL, nil)
	default:
		req, _ = http.NewRequest(http.MethodPost, sub.URL, data)
		req.Header.Set("Content-Type", contentType)
	}
	return
}

func (h *hook) sendToSub(sub *Subscriber, form *Form) {
	// Если превышен счетчик отправок у подписчика, то автоматически отписываем его (удаляем из БД)
	if sub.ErrCount >= maxErrCount {
		sub.incErrCount()
		return
	}

	var err error
	var resp *http.Response
	var req *http.Request
	req, err = newRequest(sub, form)
	if err != nil {
		log.Printf(errorLog, err)
		return
	}

	resp, err = h.client.Do(req)
	if err != nil {
		// В случае ошибки - сразу выход
		log.Printf("hook: error sending request, url='%s' error='%v'", sub.URL, err)

		// Это значит, что хост недоступен попробуем еще раз
		if strings.Contains(err.Error(), "connection refused") {
			go h.resendToSub(sub, form)
		}
		return
	}

	switch resp.StatusCode / 100 {
	case 2:
		// При положительном ответе сбрасываем счетчик ошибок обратно до 0
		sub.resetErrCount()
	case 4:
		// Если вернулся 4xx код, значит хост существует, а URL указан некорректно. Можем сразу удалять такой
		_, err = h.service.pg.Exec(sqlDeleteSub, h.name, sub.URL)
		if err != nil {
			log.Printf(hookErr, h.name, fmt.Sprintf("cannot delete subscription url='%s'", sub.URL))
		}
		log.Printf(hookWarning, h.name, fmt.Sprintf("subscription url='%s' deleted cause status code 4xx received", sub.URL))
	case 5:
		// Если вернулся 5xx код, то нужно это отметить в БД и попробовать повторить отправку через минуту
		go h.resendToSub(sub, form)
	default:
		// Нестандартное поведение логируем
		log.Printf("hook: error sending request, url='%s' error='unexpected status code %d'", sub.URL, resp.StatusCode)
	}
}

func (h *hook) resendToSub(sub *Subscriber, form *Form) {
	var err error
	var resp *http.Response
	var req *http.Request
	req, err = newRequest(sub, form)
	if err != nil {
		log.Printf(errorLog, err)
		return
	}

	resp, err = h.client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		sub.incErrCount()
	}
}

type HookFuncMap map[string]HookFunc

func NewHookFuncMap() *HookFuncMap {
	return &HookFuncMap{}
}

func (h HookFuncMap) Add(name string, function func() *Form) {
	h[name] = HookFunc{name, function}
}

func (h HookFuncMap) Delete(name string) {
	delete(h, name)
}

type HookFunc struct {
	Name     string
	Function func() *Form
}

type Form struct {
	Payload     map[string]string
	ContentType string
}

func NewForm() *Form {
	return &Form{
		Payload:     map[string]string{},
		ContentType: "",
	}
}

func (f *Form) Data() (buf *bytes.Buffer, ContentType string, err error) {
	buf = &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for k, v := range f.Payload {
		if err = w.WriteField(k, v); err != nil {
			return nil, "", err
		}
	}
	if err = w.Close(); err != nil {
		log.Printf(warningLog, "error while closing multipart.Writer")
	}
	ContentType = w.FormDataContentType()
	return
}

func (f *Form) Add(key, value string) {
	f.Payload[key] = value
}
