package service

import (
	"context"
	"fmt"
	"github.com/jackc/pgx"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Service struct {
	name   string
	server *http.Server
	WPool  *WorkerPool // Background workers
	HPool  *HookPool   // Hooks

	// Postgres
	pgURL  string
	pgConf pgx.ConnConfig
	pg     *pgx.ConnPool
}

func NewService(name string, server *http.Server, pgURL string) *Service {
	return &Service{
		name:   name,
		server: server,
		WPool:  NewPorkersPool(),
		pgURL:  pgURL,
	}
}

const startingErr = "service: cannot start service '%s' error='%v'"
const stoppingErr = "service: cannot stop service '%s' error='%v'"

// Start - Запуск сервиса. В случае ошибки вызывает panic
func (s *Service) Start(cert, key string) (err error) {
	// Подключение к БД
	if s.pgConf, err = pgx.ParseConnectionString(s.pgURL); err != nil {
		panic(fmt.Errorf(startingErr, s.name, err))
	}
	if s.pg, err = pgx.NewConnPool(pgx.ConnPoolConfig{MaxConnections: 10, ConnConfig: s.pgConf}); err != nil {
		panic(fmt.Errorf(startingErr, s.name, err))
	}

	// Добавление недостающих таблиц
	if err = s.fillDatabase(); err != nil {
		panic(fmt.Errorf(startingErr, s.name, err))
	}

	// Добавление обработчиков

	// Запуск сервера
	go func() {
		if cert != "" && key != "" {
			err = s.server.ListenAndServeTLS(cert, key)
		} else {
			err = s.server.ListenAndServe()
		}

		if err != nil {
			panic(fmt.Errorf(startingErr, s.name, err))
		}
	}()

	// Запуск воркеров
	s.WPool.StartAll()

	log.Printf("service: name='%s' has been started\n", s.name)

	// Позаботимся о перехвате прерываний для корректной остановки сервиса
	stop := make(chan os.Signal)
	signal.Notify(stop, syscall.SIGKILL, syscall.SIGSTOP, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGABRT)
	for {
		// вечный цикл ожидания
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

func (s *Service) Stop() (err error) {
	s.WPool.StopAll()

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = s.server.Shutdown(ctxShutDown); err != nil {
		return err
	}

	log.Printf("service: name='%s' has been stopped\n", s.name)
	return
}

func (s *Service) DB() *pgx.ConnPool {
	return s.pg
}

func (s *Service) fillDatabase() (err error) {
	if _, err = s.DB().Exec(sqlCreateHookTables); err != nil {
		return
	}

	return
}
