package service

import (
	"fmt"
	"log"
	u "net/url"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// hookPool - пул веб-хуков
type hookPool struct {
	parent *Service
	hooks  map[string]*hook
	sync.Mutex
}

func newHookPool(parent *Service) *hookPool {
	return &hookPool{
		parent: parent,
		hooks:  map[string]*hook{},
		Mutex:  sync.Mutex{},
	}
}

func (h *hookPool) add(hook *hook) {
	err := h.createHook(hook.name, hook.function.Name)
	if err != nil {
		log.Println(err)
		return
	}

	h.addNoDB(hook)
}

func (h *hookPool) addNoDB(hook *hook) {
	h.Lock()
	defer h.Unlock()
	h.hooks[hook.name] = hook
}

func (h *hookPool) delete(name string) {
	err := h.deleteHook(name)
	if err != nil {
		log.Println(err)
		return
	}

	h.Lock()
	defer h.Unlock()
	delete(h.hooks, name)
}

func (h *hookPool) triggerByName(name string) {
	h.Lock()
	defer h.Unlock()
	if h.hooks[name] != nil {
		if err := h.hooks[name].trigger(); err != nil {
			log.Printf(hookErr, name, err)
		}
	} else {
		log.Printf(hookErr, name, "this hook not exists")
	}
}

func (h *hookPool) createHook(name string, functionName string) (err error) {
	if _, err = h.parent.pg.Exec(sqlAddHook, name, functionName); err != nil {
		return fmt.Errorf(hookErr, name, err.Error())
	}
	return
}

func (h *hookPool) deleteHook(name string) (err error) {
	if _, err = h.parent.pg.Exec(sqlDeleteHook, name); err != nil {
		return fmt.Errorf(hookErr, name, err.Error())
	}
	return
}

func (h *hookPool) checkSubArgs(name, url, passCode string) (err error) {
	if !validName(name) {
		return fmt.Errorf(hookErr, name, fmt.Sprintf("incorrect parameter name='%s'", name))
	}

	if h.hooks[name] == nil {
		return fmt.Errorf(hookErr, name, "this hook not exists")
	}

	_, err = u.Parse(url)
	if err != nil {
		return fmt.Errorf(hookErr, name, fmt.Sprintf("incorrect parameter url='%s', error='%v'", url, err))
	}

	if passCode != "" {
		_, err = uuid.Parse(passCode)
		if err != nil {
			return fmt.Errorf(hookErr, name, fmt.Sprintf("incorrect parameter pass_code='%s', error='%v'", url, err))
		}
	}

	return nil
}

func (h *hookPool) subscribe(name string, url string) (passCode string, err error) {
	if err = h.checkSubArgs(name, url, ""); err != nil {
		return "", err
	}

	passCode = uuid.New().String()
	_, err = h.parent.pg.Exec(sqlSubscribe, name, url, passCode)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			err = fmt.Errorf("subscription allready exists")
		}
		return "", err
	}

	return
}

func (h *hookPool) unsubscribe(name, url, passCode string) (err error) {
	if err = h.checkSubArgs(name, url, passCode); err != nil {
		return err
	}

	// Перед удалением нужно проверить, а есть ли вообще такая подписка,
	// потому что при удалении несуществующей строки ошибка не возникает
	row := h.parent.pg.QueryRow(sqlSelectSubCode, name, url)
	var pgPassCode string
	err = row.Scan(&pgPassCode)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			err = fmt.Errorf("subscription not exists")
		}
		return
	}

	if pgPassCode != passCode {
		return fmt.Errorf("invalid pass_code")
	}

	_, err = h.parent.pg.Exec(sqlUnsubscribe, name, url, passCode)
	if err != nil {
		return err
	}

	return
}
