package internal

import (
	"log"
	"sync"
)

type RequestMap struct {
	rMutex sync.Mutex
	rMap   map[int64]string
}

func NewRequestMap() *RequestMap {
	return &RequestMap{rMap: make(map[int64]string)}
}

func (m *RequestMap) Insert(reqid int64, method string) {
	if reqid < 0 || len(method) < 1 {
		panic("RequestMap: insert must be called with non-nil and non-empty reqid and method")
	}
	m.rMutex.Lock()
	defer m.rMutex.Unlock()
	m.rMap[reqid] = method
}

func (m *RequestMap) Pop(reqid int64) string {
	m.rMutex.Lock()
	defer m.rMutex.Unlock()
	method, ok := m.rMap[reqid]
	if !ok || len(method) < 1 {
		// TODO: this shouldn't happen but I see it happen. why?
		log.Printf("RequestMap: reqid [%v] did not exist in map. Pop must be called after Insert.", reqid)
		return ""
	}
	delete(m.rMap, reqid)
	return method
}
