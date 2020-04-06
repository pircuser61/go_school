package model

import (
	"fmt"
	"strings"
	"sync"
)

type BTSStore struct {
	bts map[string]BTS
	mu  sync.Mutex
}

func (b *BTSStore) Lock(bts string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if stat, ok := b.bts[bts]; ok {
		if stat.blocked {
			return fmt.Sprintf("FAILED: BTS %s already blocked", bts)
		}
		stat.blocked = true
		b.bts[bts] = stat
		return fmt.Sprintf("SUCCESS: BTS %s blocked", bts)
	}
	return fmt.Sprintf("FAILED: BTS %s not found", bts)
}

func (b *BTSStore) Unlock(bts string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if stat, ok := b.bts[bts]; ok {
		if !stat.blocked {
			return fmt.Sprintf("FAILED: BTS %s already unblocked", bts)
		}
		stat.blocked = false
		b.bts[bts] = stat
		return fmt.Sprintf("SUCCESS: BTS %s unblocked", bts)
	}
	return fmt.Sprintf("FAILED: BTS %s not found", bts)
}

func (b *BTSStore) Address(bts string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if stat, ok := b.bts[bts]; ok {
		return stat.name
	}
	return fmt.Sprintf("FAILED: BTS %s not found", bts)
}

type BTS struct {
	blocked bool
	name    string
}

var datastore = BTSStore{mu: sync.Mutex{}, bts: map[string]BTS{
	"BTS_69_22222": {blocked: true, name: "ТверьГеоФизика, крыша"},
	"BTS_23_11111": {blocked: false, name: "Краснодарский край, ст. Староминская, ул.Ярмарочная"},
	"BTS_77_00001": {blocked: false, name: "г. Москва, Смоленская-Сенная, 27к2"},
}}

func RunParse(alert string) (string, string, string) {
	alertSlice := strings.Split(alert, "__")
	return get(alertSlice, 0), get(alertSlice, 1), get(alertSlice, 2)
}

func RunConnect(one, two, three string) string {
	a := []string{three, two, one}
	alert := strings.Join(a, "__")
	return alert
}

func CheckLock(s string) bool {
	if s == "LOCK" {
		return true
	}
	return false
}

func CheckUnlock(s string) bool {
	if s == "UNLOCK" {
		return true
	}
	return false
}
