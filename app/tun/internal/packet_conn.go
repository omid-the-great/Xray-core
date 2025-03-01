package internal

import (
	"fmt"

	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

type PacketConn struct {
	GvisorUDPConn *gonet.UDPConn
}

func (p *PacketConn) ReadMultiBuffer() (buf.MultiBuffer, error) {
	packet := buf.New()
	packet.Extend(buf.Size)

	n, addr, err := p.GvisorUDPConn.ReadFrom(packet.Bytes())
	if err != nil {
		packet.Release()
		return nil, err
	}

	packet.Resize(0, int32(n))

	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		packet.Release()
		return nil, fmt.Errorf("expected *net.UDPAddr, got %T", addr)
	}

	destination := net.UDPDestination(net.IPAddress(udpAddr.IP), net.Port(udpAddr.Port))

	packet.UDP = &destination

	return buf.MultiBuffer{packet}, nil
}

func (p *PacketConn) WriteMultiBuffer(mb buf.MultiBuffer) error {
	defer buf.ReleaseMulti(mb)
	for _, buffer := range mb {
		_, err := p.GvisorUDPConn.WriteTo(buffer.Bytes(), buffer.UDP.RawNetAddr())
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *PacketConn) GetConn() net.Conn {
	return p.GvisorUDPConn
}
