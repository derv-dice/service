package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gocraft/web"
	"github.com/jackc/pgx"
)

const (
	serviceErr  = "service: name='%s' error='%v'"
	startingErr = "service: cannot start service '%s' error='%v'"
	stoppingErr = "service: cannot stop service '%s' error='%v'"

	hookErr       = "hook: name='%s' error='%v'"
	hookWarning   = "hook: name='%s' warning='%s'"
	workerErr     = "worker: name='%s' error='%v'"
	workerWarning = "worker: name='%s' warning='%s'"

	warningLog = "warning: %v"
	errorLog   = "error: %v"
)

// Service - фасад, предоставляющий все методы по настройке, запуску и управлению отдельными частями сервиса
type Service struct {
	name   string          // Наименование сервиса
	server *http.Server    // Веб-сервер
	wPool  *workerPool     // Фоновые воркеры
	hPool  *hookPool       // Веб-хуки
	pgURL  string          // Postgres URL
	pgConf *pgx.ConnConfig // Информация о подлюченной БД
	pg     *pgx.ConnPool   // Пул коннектов к БД

	hFuncMap           *HookFuncMap
	deferredAddHook    map[string]string // Список отложенных добавлений хуков map[name]function_name
	deferredDeleteHook map[string]bool   // Список отложенных удалений хуков map[name]function_name
	started            bool
}

func (s *Service) Name() string      { return s.name }
func (s *Service) DB() *pgx.ConnPool { return s.pg }

// New - Конструктор нового сервиса
func New(name string, serverCfg Config, pgURL string, funcMap *HookFuncMap) (s *Service, err error) {
	// Выставление значений по умолчанию для пустых параметров
	if name == "" {
		name = "default"
	}

	if serverCfg.Mux == nil {
		serverCfg.Mux = web.New(ApiContext{})
	}

	// Валидация аргументов функции
	if err = checkArgs(name, serverCfg, pgURL); err != nil {
		return nil, fmt.Errorf(serviceErr, "", err.Error())
	}

	// Формирование нового объекта Service
	s = &Service{
		name: name,
		server: &http.Server{
			Addr:              serverCfg.Addr,
			ReadHeaderTimeout: serverCfg.ReadHeaderTimeout,
			IdleTimeout:       serverCfg.IdleTimeout,
			ReadTimeout:       serverCfg.ReadTimeout,
			WriteTimeout:      serverCfg.WriteTimeout,
			MaxHeaderBytes:    serverCfg.MaxHeaderBytes,
		},
		pgURL:              pgURL,
		pgConf:             &pgx.ConnConfig{},
		hFuncMap:           funcMap,
		deferredAddHook:    map[string]string{},
		deferredDeleteHook: map[string]bool{},
	}

	// Регистрация обработчиков подписки/отписки на веб-хуки
	subMux := serverCfg.Mux.Subrouter(hookCtx{s: s}, "/hook")
	subMux.Post("/sub/:name", (&hookCtx{s: s}).subscribeHandler)
	subMux.Post("/unsub/:name", (&hookCtx{s: s}).unsubscribeHandler)
	s.server.Handler = serverCfg.Mux

	// Добавление пулов воркеров и веб-хуков
	s.wPool = newWorkerPool(s)
	s.hPool = newHookPool(s)
	return s, nil
}

// Start - Запуск сервиса
func (s *Service) Start(cert, key string) {
	var err error
	defer func() {
		if err != nil {
			log.Println(fmt.Errorf(startingErr, s.name, err))
		}
	}()

	// Подключение к БД
	var pgConf pgx.ConnConfig
	pgConf, err = pgx.ParseConnectionString(s.pgURL)
	if err != nil {
		return
	}

	s.pgConf = &pgConf
	if s.pg, err = pgx.NewConnPool(pgx.ConnPoolConfig{MaxConnections: 100, ConnConfig: *s.pgConf}); err != nil {
		return
	}

	// Добавление недостающих таблиц
	if err = s.configureDB(); err != nil {
		return
	}

	// Загрузка данных о существующих хуках и функциях, которые выполняются при их вызове
	if err = s.loadHooks(); err != nil {
		return
	}

	// Запуск сервера
	go func() {
		if cert != "" && key != "" {
			err = s.server.ListenAndServeTLS(cert, key)
		} else {
			err = s.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			err = fmt.Errorf(startingErr, s.name, err)
			return
		}
	}()

	s.started = true

	// Удаление отложенных веб-хуков
	wg1 := sync.WaitGroup{}
	for k := range s.deferredDeleteHook {
		wg1.Add(1)
		go func(_wg *sync.WaitGroup, _name string) {
			defer _wg.Done()
			s.DeleteHook(_name)
		}(&wg1, k)
	}
	wg1.Wait()

	// Добавление отложенных веб-хуков
	wg2 := sync.WaitGroup{}
	for k, v := range s.deferredAddHook {
		wg2.Add(1)
		go func(_wg *sync.WaitGroup, _name, _funcName string) {
			defer _wg.Done()
			s.AddHook(_name, _funcName)
		}(&wg2, k, v)
	}
	wg2.Wait()

	// Запуск воркеров только после того, как все хуки добавлены
	go s.wPool.startAll()

	log.Printf("service: Name='%s' has been started\n", s.name)

	// Позаботимся о перехвате прерываний для корректной остановки сервиса
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGKILL, syscall.SIGSTOP, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGABRT)
	for {
		select {
		case <-stop:
			if err = s.Stop(); err != nil {
				log.Printf(stoppingErr, s.name, err)
			}
			os.Exit(1)
		}
	}

	return
}

// Stop - Остановка сервиса
func (s *Service) Stop() (err error) {
	go s.wPool.stopAll()

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = s.server.Shutdown(ctxShutDown); err != nil && err != http.ErrServerClosed {
		return err
	}

	log.Printf("service: Name='%s' has been stopped\n", s.name)
	return
}

type Config struct {
	Addr              string
	Mux               *web.Router
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	MaxHeaderBytes    int
}

type ApiContext struct {
	Params map[string]interface{}
}

func (s *Service) configureDB() (err error) {
	if _, err = s.pg.Exec(createHookSchema); err != nil {
		return
	}
	return
}

func (s *Service) loadHooks() (err error) {
	var rows *pgx.Rows
	if rows, err = s.pg.Query(sqlSelectHooks); err != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		tmp := &tHook{}
		err = rows.Scan(&tmp.Name, &tmp.Function)
		if err != nil {
			return
		}

		if (*s.hFuncMap)[tmp.Function].Function == nil {
			log.Printf(serviceErr, s.name, fmt.Sprintf("cannot load hook, hookFunc name='%s' not found", tmp.Name))
			continue
		}

		s.hPool.addNoDB(newHook(tmp.Name, (*s.hFuncMap)[tmp.Function], s))
	}
	return
}

func (s *Service) deferAddHook(name, functionName string) {
	s.deferredAddHook[name] = functionName
}

func (s *Service) deferDeleteHook(name string) {
	s.deferredDeleteHook[name] = true
}

func checkArgs(name string, serverCfg Config, pgURL string) (err error) {
	if !validName(name) {
		return fmt.Errorf("invalid arg: 'Name'")
	}

	if serverCfg.Addr == "" {
		return fmt.Errorf("invalid arg: 'serverCfg.Addr'")
	}

	if pgURL == "" {
		return fmt.Errorf("invalid arg: 'pgURL'")
	}
	return
}

/* ================================================ Worker methods ================================================== */

// AddWorker - Добавить новый воркер
func (s *Service) AddWorker(name string, period time.Duration, function WorkerFunc) {
	s.wPool.add(newWorker(name, period, function))
}

// DeleteWorker - Остановка и удаление воркера
func (s *Service) DeleteWorker(name string) { s.wPool.delete(name) }

// StartWorker - Запуск воркера
func (s *Service) StartWorker(name string) { s.wPool.startByName(name) }

// StopWorker - Остановка воркера
func (s *Service) StopWorker(name string) { s.wPool.stopByName(name) }

/* ================================================= Hook methods =================================================== */

// AddHook - Добавление нового веб-хука
func (s *Service) AddHook(name, functionName string) {
	// Если сервис еще не стартовал, то добавление хука упадет с ошибкой из-за того,
	// что функции для возова хуком еще нет. Поэтому отправляем добавление хука в отложенный вызов,
	// чтобы выполнить ее после старта сервиса
	if !s.started {
		s.deferAddHook(name, functionName)
		return
	}

	if (*s.hFuncMap)[functionName].Name == "" {
		log.Printf(hookErr, name, fmt.Sprintf("cannot create hook, function name='%s' not found", functionName))
		return
	}

	s.hPool.add(newHook(name, (*s.hFuncMap)[functionName], s))
}

// DeleteHook - Удаление веб-хука. Все подписки удалятся вместе с ним
// Если вызвать перед Start(), то выполнится перед AddHook()
func (s *Service) DeleteHook(name string) {
	// Если сервис еще не стартовал, то удалить хук не получится, потому что подключения к БД еще нет
	if !s.started {
		s.deferDeleteHook(name)
		return
	}

	s.hPool.delete(name)
}

// TriggerHook - Принудательное выполнение веб-хука
func (s *Service) TriggerHook(name string) {
	s.hPool.triggerByName(name)
}

// SubscribeHook - Подписка на веб-хук
func (s *Service) SubscribeHook(name, url string) (passCode string, err error) {
	return s.hPool.subscribe(name, url)
}

// UnsubscribeHook - Отписка от веб-хука
func (s *Service) UnsubscribeHook(name, url, passCode string) (err error) {
	return s.hPool.unsubscribe(name, url, passCode)
}
