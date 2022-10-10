
package console

import (
	"github.com/kmcsr/go-logger"
	"github.com/kmcsr/go-logger/std"

	"github.com/kmcsr/goja"
	"github.com/kmcsr/goja/extern/require"
	"github.com/kmcsr/goja/extern/util"
)

const ModuleName = "node:console"

type Console struct {
	util   *goja.Object
	logger logger.Logger
}

type Function = func(call goja.FunctionCall, runtime *goja.Runtime)(goja.Value)

func (c *Console)wrap(loger func(string))(Function){
	return func(call goja.FunctionCall, runtime *goja.Runtime)(goja.Value){
		if formatter, ok := goja.AssertFunction(c.util.Get("format")); ok {
			res, err := formatter(c.util, call.Arguments...)
			if err != nil {
				panic(err)
			}
			loger(res.String())
		}else{
			panic(runtime.NewTypeError("util.format is not a function"))
		}
		return nil
	}
}

func RequireWithLogger(loger logger.Logger)(require.ModuleLoader){
	return func(runtime *goja.Runtime, module *goja.Object) {
		c := &Console{
			logger: loger,
		}

		c.util = require.Require(runtime, util.ModuleName).(*goja.Object)

		o := module.Get("exports").(*goja.Object)
		o.Set("trace", c.wrap(func(v string){ c.logger.Trace(v) }))
		o.Set("debug", c.wrap(func(v string){ c.logger.Debug(v) }))
		o.Set("info",  c.wrap(func(v string){ c.logger.Info(v) }))
		o.Set("warn",  c.wrap(func(v string){ c.logger.Warn(v) }))
		o.Set("error", c.wrap(func(v string){ c.logger.Error(v) }))
		o.Set("log", o.Get("info"))
	}
}

func Enable(runtime *goja.Runtime) {
	runtime.Set("console", require.Require(runtime, ModuleName))
}

func RegisterWithLogger(r *require.Registry, loger logger.Logger){
	r.RegisterNativeModule(ModuleName, RequireWithLogger(loger))
}

func init() {
	require.RegisterNativeModule(ModuleName, RequireWithLogger(logger.std.Logger))
}
