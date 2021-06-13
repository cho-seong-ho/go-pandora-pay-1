package multicast

import (
	"sync"
	"sync/atomic"
)

type MulticastChannel struct {
	listeners *atomic.Value //[]chan interface{}
	sync.Mutex
}

func (self *MulticastChannel) AddListener() <-chan interface{} {
	self.Lock()
	defer self.Unlock()

	listeners := self.listeners.Load().([]chan interface{})
	newChan := make(chan interface{})

	self.listeners.Store(append(listeners, newChan))

	return newChan
}

func (self *MulticastChannel) BroadcastAwait(data interface{}) {

	listeners := self.listeners.Load().([]chan interface{})

	for _, channel := range listeners {
		channel <- data
	}

}

func (self *MulticastChannel) Broadcast(data interface{}) {

	listeners := self.listeners.Load().([]chan interface{})

	for _, channel := range listeners {
		select {
		case channel <- data:
		default:
		}
	}

}

func (self *MulticastChannel) RemoveChannel(remove chan interface{}) bool {
	self.Lock()
	defer self.Unlock()

	listeners := self.listeners.Load().([]chan interface{})
	for i, channel := range listeners {
		if channel == remove {
			listeners = append(listeners[:i], listeners[:i+1]...)
			self.listeners.Store(listeners)
			return true
		}
	}

	return false
}

func (self *MulticastChannel) CloseAll() {
	self.Lock()
	defer self.Unlock()

	listeners := self.listeners.Load().([]chan interface{})
	for _, channel := range listeners {
		close(channel)
	}
	self.listeners.Store(make([]chan interface{}, 0))
}

func NewMulticastChannel() *MulticastChannel {

	multicast := &MulticastChannel{
		listeners: &atomic.Value{}, //[]chan interface{}
	}
	multicast.listeners.Store(make([]chan interface{}, 0))

	return multicast
}
