package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gocraft/web"
)

const maxMultipartMemory = 1000000 // 1Мб

type hookCtx struct {
	*ApiContext
	s *Service
}

type hookResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

func sendHookResponse(w web.ResponseWriter, code string, err error) (success bool) {
	var status int
	var resp hookResponse
	if err != nil {
		status = http.StatusBadRequest
		resp.Error = err.Error()
	} else {
		status = http.StatusOK
		resp.Code = code
		resp.Success = true
	}

	var jsonData []byte
	jsonData, err = json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("subscription: error='%v'", err)
		return false
	}

	w.WriteHeader(status)
	_, err = w.Write(jsonData)
	if err != nil {
		log.Printf("subscription: error='%v'", err)
		return false
	}

	if status == 200 {
		return true
	}
	return
}

func (h *hookCtx) subscribeHandler(w web.ResponseWriter, r *web.Request) {
	var err error
	err = r.ParseMultipartForm(maxMultipartMemory)
	if err != nil {
		http.Error(w, fmt.Sprintf("error while parsing form-data: %v", err), http.StatusBadRequest)
		return
	}

	name := r.PathParams["name"]
	url := r.PostFormValue("url")

	var code string
	code, err = h.s.SubscribeHook(name, url)
	if sendHookResponse(w, code, err) {
		log.Printf("hook: subscription success args:(hook='%s', url='%s', code='%s')", name, url, code)
	}
}

func (h *hookCtx) unsubscribeHandler(w web.ResponseWriter, r *web.Request) {
	var err error
	err = r.ParseMultipartForm(maxMultipartMemory)
	if err != nil {
		http.Error(w, fmt.Sprintf("error while parsing form-data: %v", err), http.StatusBadRequest)
		return
	}

	name := r.PathParams["name"]
	url := r.PostFormValue("url")
	passCode := r.PostFormValue("pass_code")

	err = h.s.UnsubscribeHook(name, url, passCode)
	if sendHookResponse(w, "", err) {
		log.Printf("hook: unsubscription success args:(hook='%s', url='%s', passCode='%s')", name, url, passCode)
	}
}
