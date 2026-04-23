package config

import (
	"sync"
	"time"
)

// Snapshot 配置快照
type Snapshot[T any] struct {
	Revision int64
	Data     []T
	ParsedAt time.Time
}

// TypedStore 类型安全的配置存储
type TypedStore[T any] struct {
	mu       sync.RWMutex
	snapshot *Snapshot[T]
}

// NewTypedStore 创建类型安全存储
func NewTypedStore[T any]() *TypedStore[T] {
	return &TypedStore[T]{}
}

// Get 返回快照（线程安全）
func (s *TypedStore[T]) Get() *Snapshot[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

// Update 更新快照
func (s *TypedStore[T]) Update(revision int64, data []T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = &Snapshot[T]{
		Revision: revision,
		Data:     data,
		ParsedAt: time.Now(),
	}
}
