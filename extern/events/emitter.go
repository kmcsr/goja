
package events

import (
	"reflect"

	"github.com/kmcsr/goja"
)

type listenerI interface{
	caller()(goja.Callable)
	listener()(goja.Callable)
}

type gojaCallable goja.Callable

var _ listenerI = (gojaCallable)(nil)

func (c gojaCallable)caller()(goja.Callable){
	return (goja.Callable)(c)
}

func (c gojaCallable)listener()(goja.Callable){
	return (goja.Callable)(c)
}

type gojaCallableOnce struct{
	c goja.Callable
	l goja.Callable
}

var _ listenerI = (*gojaCallableOnce)(nil)

func newGojaCallableOnce(e *EventEmitter, event string, call goja.Callable)(c *gojaCallableOnce){
	c = &gojaCallableOnce{
		c: func(this goja.Value, args ...goja.Value)(res goja.Value, err error){
			e.removeListener(event, c.c)
			return c.l(this, args...)
		},
		l: call,
	}
	return
}

func (c *gojaCallableOnce)caller()(goja.Callable){
	return c.c
}

func (c *gojaCallableOnce)listener()(goja.Callable){
	return c.c
}

type EventEmitter struct{
	runtime *goja.Runtime
	events map[string][]listenerI
	inerror bool
}

func NewEventEmitter(runtime *goja.Runtime)(e *EventEmitter){
	e = &EventEmitter{
		runtime: runtime,
		events: make(map[string][]listenerI),
	}
	return
}

func (e *EventEmitter)ToObject()(*goja.Object){
	return e.runtime.ToValue(e).(*goja.Object)
}

func (e *EventEmitter)emit(vm *goja.Runtime, event string, values ...any)(ok bool){
	calls := e.events[event]
	if len(calls) == 0 {
		return false
	}
	vals := castToJsValues(vm, values)
	for _, c := range calls {
		if _, err := c.caller()(nil, vals...); err != nil {
			if e.inerror {
				panic("Error in error handle")
			}
			e.inerror = true
			e.emit(vm, "error", vm.ToValue(err))
		}
	}
	return
}

func (e *EventEmitter)Emit(event string, values ...any)(ok bool){
	return e.emit(e.runtime, event, values...)
}

func (e *EventEmitter)EmitAsync(event string, values ...any)(*EventEmitter){
	e.runtime.RunOnLoop(func(vm *goja.Runtime)(error){
		e.emit(vm, event, values...)
		return nil
	})
	return e
}

func (e *EventEmitter)addListener(event string, listener listenerI){
	e.events[event] = append(e.events[event], listener)
}

func (e *EventEmitter)prependListener(event string, listener listenerI){
	hd1 := e.events[event]
	hd2 := make([]listenerI, len(hd1) + 1)
	hd2[0] = listener
	copy(hd2[1:], hd1)
	e.events[event] = hd2
}

func (e *EventEmitter)AddListener(event string, listener goja.Callable)(*EventEmitter){
	e.addListener(event, (gojaCallable)(listener))
	return e
}

func (e *EventEmitter)On(event string, listener goja.Callable)(*EventEmitter){
	e.AddListener(event, listener)
	return e
}

func (e *EventEmitter)Once(event string, listener goja.Callable)(*EventEmitter){
	e.addListener(event, newGojaCallableOnce(e, event, listener))
	return e
}

func (e *EventEmitter)PrependListener(event string, listener goja.Callable)(*EventEmitter){
	e.prependListener(event, (gojaCallable)(listener))
	return e
}

func (e *EventEmitter)PrependOnceListener(event string, listener goja.Callable)(*EventEmitter){
	e.prependListener(event, newGojaCallableOnce(e, event, listener))
	return e
}

func (e *EventEmitter)removeListener(event string, listener goja.Callable){
	if calls, ok := e.events[event]; ok {
		hp := reflect.ValueOf(listener).Pointer()
		for i := len(calls) - 1; i >= 0; i-- {
			if reflect.ValueOf(calls[i].caller()).Pointer() == hp ||
				 reflect.ValueOf(calls[i].listener()).Pointer() == hp {
				copy(calls[i:], calls[i + 1:])
				e.events[event] = calls[:len(calls) - 1]
				break
			}
		}
	}
}

func (e *EventEmitter)RemoveListener(event string, listener goja.Callable)(*EventEmitter){
	e.removeListener(event, listener)
	return e
}

func (e *EventEmitter)RemoveListenerAsync(event string, listener goja.Callable){
	e.runtime.DoSync(func()(error){
		e.removeListener(event, listener)
		return nil
	})
}

func (e *EventEmitter)removeAllListeners(event string){
	if _, ok := e.events[event]; ok {
		delete(e.events, event)
	}
}

func (e *EventEmitter)RemoveAllListeners(event string){
	e.runtime.DoSync(func()(error){
		e.removeAllListeners(event)
		return nil
	})
}

func (e *EventEmitter)Off(event string, handler goja.Callable){
	e.RemoveListener(event, handler)
}

func (e *EventEmitter)EventNames()(names []string){
	names = make([]string, 0, len(e.events))
	for n, _ := range e.events {
		names = append(names, n)
	}
	return
}

func (e *EventEmitter)Listeners(event string)(handlers []goja.Callable){
	hds := e.events[event]
	handlers = make([]goja.Callable, 0, len(hds))
	for _, c := range hds {
		handlers = append(handlers, c.listener())
	}
	return
}

func castToJsValues(vm *goja.Runtime, values []any)(vals []goja.Value){
	vals = make([]goja.Value, len(values))
	for i, v := range values {
		vals[i] = vm.ToValue(v)
	}
	return
}
