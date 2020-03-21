package eventbus

import (
	"container/heap"
	"fmt"
	"reflect"
	"sync"
)

type EventConfigurator func(handler EventHandler)

type EventHandler interface {
}

// SubscribeOnce subscribes to a topic once. Handler will be removed after executing.
func WithOnce() EventConfigurator {
	return func(handler EventHandler) {
		h := handler.(*eventHandler)
		h.flagOnce = true
	}
}

// SubscribeAsync subscribes to a topic with an asynchronous callback
func WithAsync() EventConfigurator {
	return func(handler EventHandler) {
		h := handler.(*eventHandler)
		h.async = true
	}
}

// Transactional determines whether subsequent callbacks for a topic are
// run serially (true) or concurrently (false)
func WithTransactional() EventConfigurator {
	return func(handler EventHandler) {
		h := handler.(*eventHandler)
		h.transactional = true
	}
}

// Order determines order of fn be call in a same topic
func WithOrder(order int) EventConfigurator {
	return func(handler EventHandler) {
		h := handler.(*eventHandler)
		h.order = order
	}
}

//BusSubscriber defines subscription-related bus behavior
type BusSubscriber interface {
	Subscribe(topic string, fn interface{}, configurators ...EventConfigurator) error
	Unsubscribe(topic string, handler interface{}, configurators ...EventConfigurator) error
}

//BusPublisher defines publishing-related bus behavior
type BusPublisher interface {
	// Publish return error if any of subscribe func return values contains error
	Publish(topic string, args ...interface{}) error
}

//BusController defines bus control behavior (checking handler's presence, synchronization)
type BusController interface {
	HasCallback(topic string) bool
	WaitAsync()
}

//Bus englobes global (subscribe, publish, control) bus behavior
type Bus interface {
	BusController
	BusSubscriber
	BusPublisher
}

// EventBus - box for handlers and callbacks.
type EventBus struct {
	handlers map[string]*eventHandlerQueue
	lock     sync.Mutex // a lock for the map
	wg       sync.WaitGroup
}

type eventHandler struct {
	callBack      reflect.Value
	flagOnce      bool
	async         bool
	transactional bool
	order         int
	sync.Mutex    // lock for an event handler - useful for running async callbacks serially
}

// newEventHandler returns new EventHandler with config
func newEventHandler(fn interface{}, configurators []EventConfigurator) *eventHandler {
	e := &eventHandler{
		callBack: reflect.ValueOf(fn),
		Mutex:    sync.Mutex{},
	}
	// config
	for i := range configurators {
		configurators[i](e)
	}
	return e
}

func (e *eventHandler) Equal(another *eventHandler) bool {
	return e.callBack == another.callBack &&
		e.flagOnce == another.flagOnce &&
		e.async == another.async &&
		e.transactional == another.transactional
}

var _ heap.Interface = &eventHandlerQueue{}

type eventHandlerQueue []*eventHandler

func (l *eventHandlerQueue) Len() int {
	return len(*l)
}

func (l *eventHandlerQueue) Less(i, j int) bool {
	return (*l)[i].order > (*l)[j].order
}

func (l *eventHandlerQueue) Swap(i, j int) {
	(*l)[i], (*l)[j] = (*l)[j], (*l)[i]
}

func (l *eventHandlerQueue) Push(x interface{}) {
	*l = append(*l, x.(*eventHandler))
}

func (l *eventHandlerQueue) Pop() interface{} {
	n := len(*l)
	v := (*l)[n-1]
	*l = (*l)[0 : n-1]
	return v
}

// New returns new EventBus with empty handlers.
func New() Bus {
	b := &EventBus{
		handlers: make(map[string]*eventHandlerQueue),
		lock:     sync.Mutex{},
		wg:       sync.WaitGroup{},
	}
	return Bus(b)
}

// doSubscribe handles the subscription logic and is utilized by the public Subscribe functions
func (bus *EventBus) doSubscribe(topic string, fn interface{}, handler *eventHandler) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	if !(reflect.TypeOf(fn).Kind() == reflect.Func) {
		return fmt.Errorf("%s is not of type reflect.Func", reflect.TypeOf(fn).Kind())
	}
	if queue, has := bus.handlers[topic]; has {
		heap.Push(queue, handler)
	} else {
		hq := &eventHandlerQueue{}
		heap.Push(hq, handler)
		bus.handlers[topic] = hq
	}
	return nil
}

// Subscribe subscribes to a topic.
// Returns error if `fn` is not a function.
func (bus *EventBus) Subscribe(topic string, fn interface{}, configurators ...EventConfigurator) error {
	e := newEventHandler(fn, configurators)
	return bus.doSubscribe(topic, fn, e)
}

// HasCallback returns true if exists any callback subscribed to the topic.
func (bus *EventBus) HasCallback(topic string) bool {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	_, ok := bus.handlers[topic]
	if ok {
		return len(*bus.handlers[topic]) > 0
	}
	return false
}

// Unsubscribe removes callback defined for a topic.
// Returns error if there are no callbacks subscribed to the topic.
func (bus *EventBus) Unsubscribe(topic string, handler interface{}, configurators ...EventConfigurator) error {
	bus.lock.Lock()
	defer bus.lock.Unlock()
	if _, ok := bus.handlers[topic]; ok && len(*bus.handlers[topic]) > 0 {
		e := newEventHandler(handler, configurators)
		bus.removeHandler(topic, bus.findEventHandlerIdx(topic, e))
		return nil
	}
	return fmt.Errorf("topic %s doesn't exist", topic)
}

// Publish executes callback defined for a topic. Any additional argument will be transferred to the callback.
func (bus *EventBus) Publish(topic string, args ...interface{}) error {
	bus.lock.Lock() // will unlock if handler is not found or always after setUpPublish
	defer bus.lock.Unlock()
	if handlers, ok := bus.handlers[topic]; ok && 0 < len(*handlers) {
		// Handlers slice may be changed by removeHandler and Unsubscribe during iteration,
		// so make a copy and iterate the copied slice.
		copyHandlers := make(eventHandlerQueue, 0, len(*handlers))
		copyHandlers = append(copyHandlers, *handlers...)
		for {
			if copyHandlers.Len() == 0 {
				return nil
			}
			h := heap.Pop(&copyHandlers)
			handler := h.(*eventHandler)
			if handler.flagOnce {
				bus.removeHandler(topic, bus.findEventHandlerIdx(topic, handler))
			}
			if !handler.async {
				if err := bus.doPublish(handler, topic, args...); err != nil {
					return err
				}
			} else {
				bus.wg.Add(1)
				if handler.transactional {
					handler.Lock()
				}
				go bus.doPublishAsync(handler, topic, args...)
			}
		}
	}
	return nil
}

var errorInterface = reflect.TypeOf((*error)(nil)).Elem()

func (bus *EventBus) doPublish(handler *eventHandler, topic string, args ...interface{}) error {
	passedArguments := bus.setUpPublish(topic, args...)
	values := handler.callBack.Call(passedArguments)
	for i := range values {
		value := values[i]
		if value.Type().Implements(errorInterface) {
			return value.Interface().(error)
		}
	}
	return nil
}

func (bus *EventBus) doPublishAsync(handler *eventHandler, topic string, args ...interface{}) {
	defer bus.wg.Done()
	if handler.transactional {
		defer handler.Unlock()
	}
	bus.doPublish(handler, topic, args...)
}

func (bus *EventBus) removeHandler(topic string, idx int) {
	if _, ok := bus.handlers[topic]; !ok {
		return
	}
	l := len(*bus.handlers[topic])

	if !(0 <= idx && idx < l) {
		return
	}

	queue := bus.handlers[topic]
	heap.Remove(queue, idx)
}

func (bus *EventBus) findEventHandlerIdx(topic string, eventHandler *eventHandler) int {
	if _, ok := bus.handlers[topic]; ok {
		for idx, handler := range *bus.handlers[topic] {
			if handler.Equal(eventHandler) {
				return idx
			}
		}
	}
	return -1
}

func (bus *EventBus) setUpPublish(topic string, args ...interface{}) []reflect.Value {
	passedArguments := make([]reflect.Value, 0)
	for _, arg := range args {
		passedArguments = append(passedArguments, reflect.ValueOf(arg))
	}
	return passedArguments
}

// WaitAsync waits for all async callbacks to complete
func (bus *EventBus) WaitAsync() {
	bus.wg.Wait()
}
