package ws

import (
	"sync/atomic"
)

// RouteBinding 单条 WS 路由在网关内的运行时绑定（不可变快照中的一条）。
type RouteBinding struct {
	Handler            WsHandlerFunc
	Public             bool
	EstablishSession   bool
}

// AtomicRoutes 不可变路由表；发布到 AtomicRouter 后不得再修改 map。
type AtomicRoutes struct {
	byMethod map[string]RouteBinding
}

// NewAtomicRoutes 深拷贝 m 供快照使用（m 为 nil 时得到空表）。
func NewAtomicRoutes(m map[string]RouteBinding) *AtomicRoutes {
	if m == nil {
		m = map[string]RouteBinding{}
	}
	cp := make(map[string]RouteBinding, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return &AtomicRoutes{byMethod: cp}
}

// Get 读路径无锁（持指针为不变式）。
func (a *AtomicRoutes) Get(method string) (RouteBinding, bool) {
	if a == nil {
		return RouteBinding{}, false
	}
	e, ok := a.byMethod[method]
	return e, ok
}

// AtomicRouter 并发安全的当前路由快照替换。
type AtomicRouter struct {
	p atomic.Pointer[AtomicRoutes]
}

func NewAtomicRouter() *AtomicRouter {
	r := &AtomicRouter{}
	r.p.Store(NewAtomicRoutes(nil))
	return r
}

// Store 发布新快照（nil 视为空表）。
func (a *AtomicRouter) Store(next *AtomicRoutes) {
	if next == nil {
		next = NewAtomicRoutes(nil)
	}
	a.p.Store(next)
}

// Get 当前快照上查询 method。
func (a *AtomicRouter) Get(method string) (RouteBinding, bool) {
	cur := a.p.Load()
	if cur == nil {
		return RouteBinding{}, false
	}
	return cur.Get(method)
}
