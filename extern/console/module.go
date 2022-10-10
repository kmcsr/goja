
package console

import (
	"github.com/kmcsr/go-logger"
	"github.com/kmcsr/go-logger/std"

	"github.com/kmcsr/goja"
	"github.com/kmcsr/goja/extern/require"
	"github.com/kmcsr/goja/extern/util"
)

const ModuleName = "node:console"

type Module struct{
	util   *goja.Object
	Logger logger.Logger
}

var Default = &Module{
	Logger: std_logger.Logger,
}

type Function = func(call goja.FunctionCall, runtime *goja.Runtime)(goja.Value)

func (c *Module)wrap(loger func(string))(Function){
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

func (c *Module)Require(runtime *goja.Runtime, module *goja.Object){
	if c.util == nil {
		c.util = require.Require(runtime, util.ModuleName).(*goja.Object)
	}
	o := module.Get("exports").(*goja.Object)
	o.Set("trace", c.wrap(func(v string){ c.Logger.Trace(v) }))
	o.Set("debug", c.wrap(func(v string){ c.Logger.Debug(v) }))
	o.Set("info",  c.wrap(func(v string){ c.Logger.Info(v) }))
	o.Set("warn",  c.wrap(func(v string){ c.Logger.Warn(v) }))
	o.Set("error", c.wrap(func(v string){ c.Logger.Error(v) }))
	o.Set("log", o.Get("info"))
}

func RequireWithLogger(loger logger.Logger)(require.ModuleLoader){
	c := &Module{
		Logger: loger,
	}
	return c.Require
}

func Require(runtime *goja.Runtime, module *goja.Object){
	Default.Require(runtime, module)
}

func Enable(runtime *goja.Runtime) {
	runtime.Set("console", require.Require(runtime, ModuleName))
}

func RegisterWithLogger(r *require.Registry, loger logger.Logger){
	r.RegisterNativeModule(ModuleName, RequireWithLogger(loger))
}

func init(){
	require.RegisterNativeModule(ModuleName, Require)
}
