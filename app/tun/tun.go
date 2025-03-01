package tun

import (
	"context"
	"fmt"
	"github.com/xtls/xray-core/app/tun/internal"
	_ "github.com/xtls/xray-core/app/tun/internal/wintun"
	"github.com/xtls/xray-core/common"
	c "github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/session"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/features/routing"
	featuresTun "github.com/xtls/xray-core/features/tun"
	"github.com/xtls/xray-core/transport"
	"net/netip"
	"os"
	"strings"
)

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, cfg interface{}) (interface{}, error) {
		return New(ctx, cfg.(*Config))
	}))
}

type Tun struct {
	name       string
	ctx        context.Context
	dispatcher routing.Dispatcher
	tunIf      internal.Tun
	stack      *internal.Stack
}

func New(ctx context.Context, config *Config) (*Tun, error) {
	instance := core.MustFromContext(ctx)
	tun := &Tun{
		ctx:        ctx,
		dispatcher: instance.GetFeature(routing.DispatcherType()).(routing.Dispatcher),
		name:       config.InterfaceName,
	}

	var mtu uint32
	if config.Mtu == 0 {
		mtu = 9000
	} else {
		mtu = config.Mtu
	}

	tunIf, err := internal.New(config.InterfaceName, netip.MustParsePrefix(config.Inet4Address), mtu, config.AutoRoute)
	if err != nil {
		return nil, err
	}

	tun.tunIf = tunIf
	tun.stack = internal.NewStack(ctx, tunIf, tun, mtu)

	errors.LogInfo(ctx, "tun created at ", tun.name)

	return tun, nil
}

func (t *Tun) Start() error {
	err := t.stack.Start()
	if err != nil {
		return err
	}
	errors.LogInfo(t.ctx, "tun started at ", t.name)
	return nil
}

func (t *Tun) Type() interface{} {
	return featuresTun.InterfaceType()
}

func (t *Tun) Close() error {
	t.stack.Close()
	err := t.tunIf.Close()
	if err != nil {
		return err
	}
	return nil
}

func (t *Tun) NewTcpConnection(ctx context.Context, conn internal.IConnWrapper, source net.Addr, destination net.Addr) {
	sid := session.NewID()
	ctx = c.ContextWithID(ctx, sid)

	errors.LogInfo(ctx, "inbound connection from ", source)
	errors.LogInfo(ctx, "inbound connection to ", destination)

	file, _ := os.OpenFile("tcp_conns.log", os.O_APPEND|os.O_CREATE, 0666)
	builder := strings.Builder{}
	builder.WriteString("--------------------- PACKET START ---------------------\n")
	builder.WriteString(fmt.Sprintf("inbound connection from %s\n", source))
	builder.WriteString(fmt.Sprintf("inbound connection to %s\n", destination))
	file.WriteString(builder.String())

	ctx = session.ContextWithInbound(ctx, &session.Inbound{
		Source: net.DestinationFromAddr(source),
		Conn:   conn.GetConn(),
	})

	_ = t.dispatcher.DispatchLink(ctx, net.DestinationFromAddr(destination), &transport.Link{
		Reader: conn,
		Writer: conn,
	})

	conn.GetConn().Close()
}

func (t *Tun) NewUdpConnection(ctx context.Context, conn internal.IConnWrapper, source net.Addr, destination net.Addr) {
	sid := session.NewID()
	ctx = c.ContextWithID(ctx, sid)

	errors.LogInfo(ctx, "inbound packet connection from ", source)
	errors.LogInfo(ctx, "inbound packet connection to ", destination)

	file, _ := os.OpenFile("udp_conns.log", os.O_APPEND|os.O_CREATE, 0666)
	builder := strings.Builder{}
	builder.WriteString("--------------------- PACKET START ---------------------\n")
	builder.WriteString(fmt.Sprintf("inbound connection from %s\n", source))
	builder.WriteString(fmt.Sprintf("inbound connection to %s\n", destination))
	file.WriteString(builder.String())

	ctx = session.ContextWithInbound(ctx, &session.Inbound{
		Source: net.DestinationFromAddr(source),
	})

	_ = t.dispatcher.DispatchLink(ctx, net.DestinationFromAddr(destination), &transport.Link{
		Reader: conn,
		Writer: conn,
	})

	conn.GetConn().Close()
}
