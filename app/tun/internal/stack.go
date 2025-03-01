package internal

import (
	"context"
	"github.com/xtls/xray-core/common/errors"
	//"gvisor.dev/gvisor/pkg/syserr"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

type Stack struct {
	ctx     context.Context
	tun     Tun
	handler Handler
	stack   *stack.Stack
	tunMtu  uint32
}

func NewStack(ctx context.Context, tun Tun, handler Handler, tunMtu uint32) *Stack {
	return &Stack{
		ctx:     ctx,
		tun:     tun,
		handler: handler,
		tunMtu:  tunMtu,
	}
}

func (s *Stack) Start() error {
	linkEndpoint, err := s.tun.NewEndpoint()
	if err != nil {
		return err
	}

	ipStack := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
	})

	tErr := ipStack.CreateNIC(1, linkEndpoint)
	if tErr != nil {
		return errors.New("create nic: ", tErr)
	}

	ipStack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: 1},
		{Destination: header.IPv6EmptySubnet, NIC: 1},
	})

	tErr = ipStack.SetSpoofing(1, true)
	if tErr != nil {
		return errors.New(tErr.String())
	}

	tErr = ipStack.SetPromiscuousMode(1, true)
	if tErr != nil {
		return errors.New(tErr.String())
	}

	sOpt := tcpip.TCPSACKEnabled(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &sOpt)
	mOpt := tcpip.TCPModerateReceiveBufferOption(true)
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &mOpt)

	bufSize := 20 * 1024
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPReceiveBufferSizeRangeOption{
		Min:     1,
		Default: bufSize,
		Max:     bufSize,
	})
	ipStack.SetTransportProtocolOption(tcp.ProtocolNumber, &tcpip.TCPSendBufferSizeRangeOption{
		Min:     1,
		Default: bufSize,
		Max:     bufSize,
	})

	tcpForwarder := tcp.NewForwarder(ipStack, 0, 1024, func(r *tcp.ForwarderRequest) {
		var wq waiter.Queue
		endpoint, err := r.CreateEndpoint(&wq)
		if err != nil {
			r.Complete(true)
			return
		}
		r.Complete(false)
		endpoint.SocketOptions().SetKeepAlive(true)
		tcpConn := gonet.NewTCPConn(&wq, endpoint)
		lAddr := tcpConn.RemoteAddr()
		rAddr := tcpConn.LocalAddr()
		if lAddr == nil || rAddr == nil {
			tcpConn.Close()
			return
		}
		go func() {
			s.handler.NewTcpConnection(s.ctx, &TcpConn{GvisorTCPConn: tcpConn}, lAddr, rAddr)
		}()
	})
	ipStack.SetTransportProtocolHandler(tcp.ProtocolNumber, func(id stack.TransportEndpointID, buffer *stack.PacketBuffer) bool {
		return tcpForwarder.HandlePacket(id, buffer)
	})
	udpForwarder := udp.NewForwarder(ipStack, func(request *udp.ForwarderRequest) {
		var wq waiter.Queue
		endpoint, err := request.CreateEndpoint(&wq)
		if err != nil {
			return
		}
		udpConn := gonet.NewUDPConn(&wq, endpoint)
		lAddr := udpConn.RemoteAddr()
		rAddr := udpConn.LocalAddr()
		if lAddr == nil || rAddr == nil {
			udpConn.Close()
			return
		}

		go func() {
			s.handler.NewUdpConnection(s.ctx, &PacketConn{GvisorUDPConn: udpConn}, lAddr, rAddr)
		}()
	})
	ipStack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)
	s.stack = ipStack
	return nil
}

func (s *Stack) Close() {
	s.stack.Close()
}
