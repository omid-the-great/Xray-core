package internal

import (
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type WintunEndpoint struct {
	tun        *WinTun
	dispatcher stack.NetworkDispatcher
}

func (e *WintunEndpoint) MTU() uint32 {
	return e.tun.mtu
}

func (e *WintunEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (e *WintunEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (e *WintunEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityNone
}

func (e *WintunEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	if dispatcher == nil && e.dispatcher != nil {
		e.dispatcher = nil
		return
	}
	if dispatcher != nil && e.dispatcher == nil {
		e.dispatcher = dispatcher
		go e.dispatchLoop()
	}
}

func (e *WintunEndpoint) dispatchLoop() {
	for {
		var packetBuffer buffer.Buffer

		packetBytes, err := e.tun.Read()
		if err != nil {
			break
		}

		packetBuffer = buffer.MakeWithData(packetBytes)

		ihl, ok := packetBuffer.PullUp(0, 1)
		if !ok {
			packetBuffer.Release()
			continue
		}

		var networkProtocol tcpip.NetworkProtocolNumber
		switch header.IPVersion(ihl.AsSlice()) {
		case header.IPv4Version:
			networkProtocol = header.IPv4ProtocolNumber
		case header.IPv6Version:
			networkProtocol = header.IPv6ProtocolNumber
		default:
			e.tun.Write(packetBuffer.Flatten())
			packetBuffer.Release()
			continue
		}
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload:           packetBuffer,
			IsForwardedPacket: true,
		})
		dispatcher := e.dispatcher
		if dispatcher == nil {
			pkt.DecRef()
			return
		}
		dispatcher.DeliverNetworkPacket(networkProtocol, pkt)
		pkt.DecRef()
	}
}

func (e *WintunEndpoint) IsAttached() bool {
	return e.dispatcher != nil
}

func (e *WintunEndpoint) Wait() {
}

func (e *WintunEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (e *WintunEndpoint) ParseHeader(ptr *stack.PacketBuffer) bool {
	return true
}

func (e *WintunEndpoint) AddHeader(buffer *stack.PacketBuffer) {
}

func (e *WintunEndpoint) WritePackets(packetBufferList stack.PacketBufferList) (int, tcpip.Error) {
	var n int
	for _, packet := range packetBufferList.AsSlice() {
		packetSlices := packet.AsSlices()
		var packetBytes []byte

		for _, packetSlice := range packetSlices {
			packetBytes = append(packetBytes, packetSlice...)
		}

		_, err := e.tun.Write(packetBytes)
		if err != nil {
			return n, &tcpip.ErrAborted{}
		}
		n++
	}
	return n, nil
}
