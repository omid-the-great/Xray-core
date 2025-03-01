package internal

import (
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/net"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
)

type TcpConn struct {
	GvisorTCPConn *gonet.TCPConn
}

func (t *TcpConn) ReadMultiBuffer() (buf.MultiBuffer, error) {
	buffer, err := buf.ReadBuffer(t.GvisorTCPConn)
	if err != nil {
		return nil, err
	}
	return buf.MultiBuffer{buffer}, nil
}

func (t *TcpConn) WriteMultiBuffer(bufferList buf.MultiBuffer) error {
	defer buf.ReleaseMulti(bufferList)

	for _, buffer := range bufferList {
		_, err := t.GvisorTCPConn.Write(buffer.Bytes())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TcpConn) GetConn() net.Conn {
	return t.GvisorTCPConn
}
