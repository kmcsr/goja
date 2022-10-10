
package process

import (
	"os"
	"strings"

	"github.com/kmcsr/goja"
	"github.com/kmcsr/goja/extern/require"
	"github.com/kmcsr/goja/extern/events"
)

type Process struct {
	*events.EventEmitter
	inerr bool
	env map[string]string
}

func Require(runtime *goja.Runtime, module *goja.Object){
	p := &Process{
		EventEmitter: events.NewEventEmitter(runtime),
		env: make(map[string]string),
	}

	for _, e := range os.Environ() {
		i := strings.IndexByte(e, '=')
		p.env[e[:i]] = e[i+1:]
	}

	runtime.Onerror = func(err error){
		if p.inerr {
			panic("Error in error handle")
		}
		p.inerr = true
		p.Emit("error", runtime.ToValue(err))
		p.inerr = false
	}

	o := module.Get("exports").(*goja.Object)
	emitter := p.EventEmitter.ToObject()
	o.SetPrototype(emitter)
	o.Set("events", emitter)
	o.Set("env", p.env)
}

func Enable(runtime *goja.Runtime) {
	runtime.Set("process", require.Require(runtime, "process"))
}

func init() {
	require.RegisterNativeModule("process", Require)
}
