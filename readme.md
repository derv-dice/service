# Go service

### Install: `go get github.com/derv-dice/service`

### Scheme:
![scheme.png](scheme.png)

### Using example:
```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/derv-dice/service"
	"github.com/gocraft/web"
)

func main() {
	mux := web.New(service.ApiContext{})
	mux.Get("/", func(w web.ResponseWriter, r *web.Request) {
		w.Write([]byte("hello from service"))
	})

	functions := service.NewHookFuncMap()
	functions.Add("fun_1", func() *service.Form {
		form := service.NewForm()
		form.Add("time", "wake up")
		return form
	})
	functions.Add("fun_2", func() *service.Form {
		form := service.NewForm()
		form.Add("time", time.Now().Format(time.RFC822))
		return form
	})

	s, err := service.New(
		"",
		service.Config{
			Addr: "localhost:8080",
			Mux:  mux,
		},
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s", "derv_dice", "test123", "localhost", 5432, "service_test"),
		functions,
	)

	if err != nil {
		log.Fatal(err)
	}

	s.DeleteHook("hook_1")
	s.AddHook("hook_1", "fun_1")
	s.AddWorker("hook_1_trigger", time.Second*5, func() (_ error) {
		s.TriggerHook("hook_1")
		return
	})

	s.DeleteHook("hook_2")
	s.AddHook("hook_2", "fun_2")
	s.AddWorker("hook_2_trigger", time.Second*5, func() (_ error) {
		s.TriggerHook("hook_2")
		return
	})

	go s.Start("", "")
	select {}
}
```