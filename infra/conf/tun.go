package conf

import (
	"github.com/xtls/xray-core/app/tun"
	"google.golang.org/protobuf/proto"
)

type TunConfig struct {
	InterfaceName string `json:"interfaceName,omitempty"`
	MTU           uint32 `json:"mtu,omitempty"`
	Inet4Address  string `json:"inet4Address,omitempty"`
	//Inet6Address  string `json:"inet6Address,omitempty"`
	AutoRoute bool `json:"autoRoute,omitempty"`
}

func (t *TunConfig) Build() (proto.Message, error) {
	return &tun.Config{
		InterfaceName: t.InterfaceName,
		Inet4Address:  t.Inet4Address,
		//Inet6Address:  t.Inet6Address,
		Mtu:       t.MTU,
		AutoRoute: t.AutoRoute,
	}, nil
}
