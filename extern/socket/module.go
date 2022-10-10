
package socket

import (
	"errors"
	"fmt"
	"net"

	"github.com/kmcsr/goja"
	"github.com/kmcsr/goja/extern/require"
)

const ModuleName = "socket"

var (
	NoListenErr = errors.New("Module not enable to listen")
	NoDialErr   = errors.New("Module not enable to dial")
)

type NetworkChecker = func(network string, mode string)(bool)

func NetworkOnList(networks ...string)(NetworkChecker){
	return func(network string, mode string)(bool){
		for _, n := range networks {
			if n == network {
				return true
			}
		}
		return false
	}
}

func NetworkNotOnList(networks ...string)(NetworkChecker){
	return func(network string, mode string)(bool){
		for _, n := range networks {
			if n == network {
				return false
			}
		}
		return true
	}
}

type ListenerChecker = func(network string, addr string)(bool)

func AllowListenOn(hosts ...string)(ListenerChecker){
	return func(network string, addr string)(bool){
		host, _, _ := net.SplitHostPort(addr)
		for _, a := range hosts {
			if a == host {
				return true
			}
		}
		return false
	}
}

func DisallowListenOn(hosts ...string)(ListenerChecker){
	return func(network string, addr string)(bool){
		host, _, _ := net.SplitHostPort(addr)
		for _, a := range hosts {
			if a == host {
				return false
			}
		}
		return true
	}
}

type DialerChecker = func(network string, laddr, raddr string)(bool)

func AllowDialTo(hosts ...string)(DialerChecker){
	return func(network string, laddr, raddr string)(bool){
		host, _, _ := net.SplitHostPort(raddr)
		for _, h := range hosts {
			if h == host {
				return true
			}
		}
		return false
	}
}

func DisallowDialTo(hosts ...string)(DialerChecker){
	return func(network string, laddr, raddr string)(bool){
		host, _, _ := net.SplitHostPort(raddr)
		for _, h := range hosts {
			if h == host {
				return false
			}
		}
		return true
	}
}

type Module struct{
	EnableListen bool
	EnableDial bool
	NetworkChecker NetworkChecker
	ListenerChecker ListenerChecker
	DialerChecker DialerChecker
}

var Default = &Module{
	EnableListen: true,
	EnableDial: true,
	NetworkChecker: nil,
	ListenerChecker: nil,
	DialerChecker: nil,
}

func (m *Module)Listen(call goja.FunctionCall, runtime *goja.Runtime)(v goja.Value){
	if !m.EnableListen {
		panic(runtime.ToValue(NoListenErr))
	}
	var (
		network string
		addr string
	)
	network = call.Arguments[0].Export().(string)
	if m.NetworkChecker != nil && !m.NetworkChecker(network, "listen") {
		panic(runtime.ToValue(fmt.Errorf("Module not allowed listen network %q", network)))
	}
	if len(call.Arguments) >= 2 {
		addr = call.Arguments[1].Export().(string)
	}
	if m.ListenerChecker != nil && !m.ListenerChecker(network, addr) {
		panic(runtime.ToValue(fmt.Errorf("Module not allowed listen on %q with network %q", addr, network)))
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
		adr, err := net.ResolveTCPAddr(network, addr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		lis, err := net.ListenTCP(network, adr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		return wrapTCPListener(runtime, lis)
	case "udp", "udp4", "udp6":
		adr, err := net.ResolveUDPAddr(network, addr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		conn, err := net.ListenUDP(network, adr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		return wrapUDPConn(runtime, conn)
	default:
		panic(runtime.ToValue("Unknown network '" + network + "'"))
	}
}

func (m *Module)Dial(call goja.FunctionCall, runtime *goja.Runtime)(v goja.Value){
	if !m.EnableDial {
		panic(runtime.ToValue(NoDialErr))
	}
	var (
		network string
		raddr string
		laddr string
	)
	network = call.Arguments[0].Export().(string)
	if m.NetworkChecker != nil && m.NetworkChecker(network, "dial") {
		panic(runtime.ToValue(fmt.Errorf("Module not allowed dial network %q", network)))
	}
	if len(call.Arguments) >= 2 {
		raddr = call.Arguments[1].Export().(string)
		if len(call.Arguments) >= 3 {
			laddr = call.Arguments[2].Export().(string)
		}
	}
	if m.DialerChecker != nil && !m.DialerChecker(network, laddr, raddr) {
		panic(runtime.ToValue(fmt.Errorf("Module not allowed dial to %q with network %q and local addr %q", raddr, network, laddr)))
	}
	switch network {
	case "tcp", "tcp4", "tcp6":
		radr, err := net.ResolveTCPAddr(network, raddr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		ladr, err := net.ResolveTCPAddr(network, laddr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		conn, err := net.DialTCP(network, ladr, radr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		return wrapTCPConn(runtime, conn)
	case "udp", "udp4", "udp6":
		radr, err := net.ResolveUDPAddr(network, raddr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		ladr, err := net.ResolveUDPAddr(network, laddr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		conn, err := net.DialUDP(network, ladr, radr)
		if err != nil {
			panic(runtime.ToValue(err))
		}
		return wrapUDPConn(runtime, conn)
	default:
		panic(runtime.ToValue("Unknown network '" + network + "'"))
	}
}

func (m *Module)Require(runtime *goja.Runtime, module *goja.Object){
	o := module.Get("exports").(*goja.Object)

	o.Set("listen", m.Listen)
	o.Set("dial", m.Dial)
}

func Require(runtime *goja.Runtime, module *goja.Object){
	Default.Require(runtime, module)
}

func RegisterNativeModule(r *require.Registry){
	if r != nil {
		r.RegisterNativeModule(ModuleName, Require)
	}else{
		require.RegisterNativeModule(ModuleName, Require)
	}
}
