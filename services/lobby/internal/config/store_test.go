package config

import (
	"sync"
	"testing"
)

func TestTypedStore_Get_Nil(t *testing.T) {
	store := NewTypedStore[int]()
	snap := store.Get()
	if snap != nil {
		t.Errorf("Get() on new store = %v, want nil", snap)
	}
}

func TestTypedStore_UpdateAndGet(t *testing.T) {
	store := NewTypedStore[int]()

	data := []int{1, 2, 3}
	store.Update(42, data)

	snap := store.Get()
	if snap == nil {
		t.Fatal("Get() returned nil after Update")
	}

	if snap.Revision != 42 {
		t.Errorf("Revision = %d, want 42", snap.Revision)
	}

	if len(snap.Data) != 3 {
		t.Errorf("Data length = %d, want 3", len(snap.Data))
	}

	if snap.Data[0] != 1 || snap.Data[1] != 2 || snap.Data[2] != 3 {
		t.Errorf("Data = %v, want [1, 2, 3]", snap.Data)
	}
}

func TestTypedStore_ConcurrentAccess(t *testing.T) {
	store := NewTypedStore[int]()

	var wg sync.WaitGroup
	numGoroutines := 100

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(rev int64) {
			defer wg.Done()
			store.Update(rev, []int{int(rev)})
		}(int64(i))
	}

	// 并发读取
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.Get()
		}()
	}

	wg.Wait()

	// 最终应该有一个有效快照
	snap := store.Get()
	if snap == nil {
		t.Error("Get() returned nil after concurrent operations")
	}
}

func TestTypedStore_UpdateReplaces(t *testing.T) {
	store := NewTypedStore[string]()

	store.Update(1, []string{"old"})
	store.Update(2, []string{"new"})

	snap := store.Get()
	if snap == nil {
		t.Fatal("Get() returned nil")
	}

	if snap.Revision != 2 {
		t.Errorf("Revision = %d, want 2", snap.Revision)
	}

	if len(snap.Data) != 1 || snap.Data[0] != "new" {
		t.Errorf("Data = %v, want [new]", snap.Data)
	}
}
