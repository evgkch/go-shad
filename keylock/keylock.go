//go:build !solution

package keylock

import "sort"

type KeyLock struct {
	mu   chan struct{}
	keys map[string]chan struct{}
}

func New() *KeyLock {
	l := &KeyLock{
		mu:   make(chan struct{}, 1),
		keys: make(map[string]chan struct{}),
	}
	l.mu <- struct{}{}
	return l
}

func (l *KeyLock) LockKeys(keys []string, cancel <-chan struct{}) (canceled bool, unlock func()) {
	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Strings(sorted)

	locked := make([]string, 0, len(sorted))

	for i := 0; i < len(sorted); {
		k := sorted[i]

		// берём глобальный мьютекс
		<-l.mu
		ch, exists := l.keys[k]
		if !exists {
			// ключ свободен — захватываем
			l.keys[k] = make(chan struct{})
			l.mu <- struct{}{}
			locked = append(locked, k)
			i++
			continue
		}
		// ключ занят — будем ждать его освобождения
		l.mu <- struct{}{}

		select {
		case <-ch:
			// ключ освободился, попробуем снова
		case <-cancel:
			// отмена — освобождаем уже захваченные
			<-l.mu
			for _, lk := range locked {
				close(l.keys[lk])
				delete(l.keys, lk)
			}
			l.mu <- struct{}{}
			return true, nil
		}
	}

	return false, func() {
		<-l.mu
		for _, k := range locked {
			close(l.keys[k])
			delete(l.keys, k)
		}
		l.mu <- struct{}{}
	}
}
