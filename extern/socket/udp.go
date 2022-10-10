
package socket

import (
	"net"

	"github.com/kmcsr/goja"
	"github.com/kmcsr/goja/extern/events"
)

type UDPConn struct{
	*events.EventEmitter

	conn *net.UDPConn
	runtime *goja.Runtime

	Local goja.Value
	Remote goja.Value
}

func wrapUDPConn(runtime *goja.Runtime, conn *net.UDPConn)(v goja.Value){
	c := &UDPConn{
		EventEmitter: events.NewEventEmitter(runtime),
		conn: conn,
		runtime: runtime,
		Local: runtime.ToValue(conn.LocalAddr().String()),
		Remote: runtime.ToValue(conn.RemoteAddr().String()),
	}
	v = runtime.ToValue(c)
	go func(){
		var (
			buf = make([]byte, 1024 * 32)
		)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				c.EmitAsync("error", c.runtime.ToValue(err))
				return
			}
			c.EmitAsync("data", c.runtime.ToValue(buf[:n]), c.runtime.ToValue(addr.String()))
		}
	}()
	return
}

func (c *UDPConn)Close(){
	if err := c.conn.Close(); err != nil {
		c.EmitAsync("error", c.runtime.ToValue(err))
		panic(c.runtime.NewGoError(err))
	}
}

func (c *UDPConn)Send(buf []byte){
	if _, err := c.conn.Write(buf); err != nil {
		c.EmitAsync("error", c.runtime.ToValue(err))
		panic(c.runtime.NewGoError(err))
	}
}

func (c *UDPConn)SendTo(buf []byte, addr string){
	adr, err := net.ResolveUDPAddr(c.conn.LocalAddr().Network(), addr)
	if err != nil {
		c.EmitAsync("error", c.runtime.ToValue(err))
		panic(c.runtime.NewGoError(err))
	}
	if _, err := c.conn.WriteToUDP(buf, adr); err != nil {
		c.EmitAsync("error", c.runtime.ToValue(err))
		panic(c.runtime.NewGoError(err))
	}
}
