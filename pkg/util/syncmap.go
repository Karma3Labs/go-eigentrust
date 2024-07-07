package util

import "sync"

type SyncMap[K, V any] struct {
	mapping sync.Map
}

func (m *SyncMap[K, V]) Load(key K) (value V, ok bool) {
	v, ok := m.mapping.Load(key)
	return v.(V), ok
}

func (m *SyncMap[K, V]) Store(key K, value V) {
	m.mapping.Store(key, value)
}

func (m *SyncMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	v, loaded := m.mapping.LoadOrStore(key, value)
	return v.(V), loaded
}

func (m *SyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, loaded := m.mapping.LoadAndDelete(key)
	return v.(V), loaded
}

func (m *SyncMap[K, V]) Delete(key K) {
	m.mapping.Delete(key)
}

func (m *SyncMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	v, loaded := m.mapping.Swap(key, value)
	return v.(V), loaded
}

func (m *SyncMap[K, V]) CompareAndSwap(key K, old, new V) bool {
	return m.mapping.CompareAndSwap(key, old, new)
}

func (m *SyncMap[K, V]) CompareAndDelete(key K, old V) (deleted bool) {
	return m.mapping.CompareAndDelete(key, old)

}

func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.mapping.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}
