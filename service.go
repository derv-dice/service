package service

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"net/http"
	"os"
)

type Service struct {
	name   string
	server *http.Server
	WPool  *WorkerPool // Background workers
	HPool  *HookPool   // Hooks

	// Postgres
	pgUrl string
	pg    *pgxpool.Pool
}

func NewService(name string, server *http.Server, pgURL string) *Service {
	return &Service{
		name:   name,
		server: server,
		WPool:  NewPorkersPool(),
		pgUrl:  pgURL,
	}
}

func (s *Service) Start(cert, key string) (err error) {
	// Подключение к БД
	s.pg, err = pgxpool.Connect(context.Background(), os.Getenv(s.pgUrl))
	if err != nil {
		return fmt.Errorf("service: can't start service '%s' error='%v'", s.name, err)
	}

	// Запуск сервера
	go func() {
		if cert != "" && key != "" {
			err = s.server.ListenAndServeTLS(cert, key)
		} else {
			err = s.server.ListenAndServe()
		}

		if err != nil {
			return
		}
	}()

	// Запуск воркеров
	s.WPool.StartAll()

	//

	return
}

func (s *Service) Stop() (err error) {
	// TODO
	return
}

func (s *Service) DB() *pgxpool.Pool {
	return s.pg
}
