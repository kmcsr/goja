
package events

import(
	"github.com/kmcsr/goja"
	"github.com/kmcsr/goja/extern/require"
)

const ModuleName = "node:events"

func newEventEmitter(call goja.ConstructorCall, runtime *goja.Runtime)(v *goja.Object){
	emitter := NewEventEmitter(runtime)
	return emitter.ToObject()
}

func Require(runtime *goja.Runtime, module *goja.Object){
	o := module.Get("exports").(*goja.Object)
	o.Set("EventEmitter", newEventEmitter)
}

func Enable(runtime *goja.Runtime) {
	runtime.Set("events", require.Require(runtime, ModuleName))
}

func Register(r *require.Registry){
	r.RegisterNativeModule(ModuleName, Require)
}

func init() {
	require.RegisterNativeModule(ModuleName, Require)
}
