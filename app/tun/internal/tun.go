package internal

import (
	"context"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type Handler interface {
	NewTcpConnection(context.Context, IConnWrapper, net.Addr, net.Addr)
	NewUdpConnection(context.Context, IConnWrapper, net.Addr, net.Addr)
}

type IConnWrapper interface {
	buf.Reader
	buf.Writer
	GetConn() net.Conn
}

type Tun interface {
	Read() (p []byte, err error)
	Write(p []byte) (n int, err error)
	Close() error
	NewEndpoint() (stack.LinkEndpoint, error)
}
