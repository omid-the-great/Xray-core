package internal

import (
	"crypto/sha256"
	"fmt"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/xtls/xray-core/app/tun/internal/winipcfg"
	"github.com/xtls/xray-core/common/errors"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

type WinTun struct {
	adapter      *wintun.Adapter
	inet4Address netip.Prefix
	//inet6Address netip.Prefix
	mtu       uint32
	autoRoute bool
	session   *wintun.Session
	readWait  windows.Handle
	closeOnce sync.Once
	rate      rateJuggler
	waitGroup sync.WaitGroup
	close     atomic.Int32
}

func New(interfaceName string, inet4Address netip.Prefix, mtu uint32, autoRoute bool) (*WinTun, error) {
	adapter, err := wintun.CreateAdapter(interfaceName, "xray-tun", generateGUIDByDeviceName(interfaceName))
	if err != nil {
		return nil, err
	}

	session, err := adapter.StartSession(0x800000)
	if err != nil {
		adapter.Close()
		return nil, err
	}

	tun := &WinTun{
		adapter:      adapter,
		inet4Address: inet4Address,
		//inet6Address: inet6Address,
		mtu:       mtu,
		autoRoute: autoRoute,
		session:   &session,
		readWait:  session.ReadWaitEvent(),
	}

	err = tun.configure()
	if err != nil {
		tun.Close()
		return nil, err
	}

	return tun, nil
}

//func (t *WinTun) configure() error {
//	luid := winipcfg.LUID(t.adapter.LUID())
//	luid.FlushIPAddresses(winipcfg.AddressFamily(windows.AF_INET))
//	luid.FlushDNS(winipcfg.AddressFamily(windows.AF_INET))
//	//if t.inet4Address.IsValid() {
//	//	err := luid.SetIPAddressesForFamily(winipcfg.AddressFamily(windows.AF_INET), []netip.Prefix{t.inet4Address})
//	//	if err != nil {
//	//		return errors.New(err, "set ipv4 address")
//	//	}
//	//}
//	//if t.inet6Address.IsValid() {
//	//	err := luid.SetIPAddressesForFamily(winipcfg.AddressFamily(windows.AF_INET6), []netip.Prefix{t.inet6Address})
//	//	if err != nil {
//	//		return errors.New(err, "set ipv6 address")
//	//	}
//	//}
//	//err := luid.SetDNS(winipcfg.AddressFamily(windows.AF_INET), []netip.Addr{t.inet4Address.Addr().Next()}, nil)
//	//if err != nil {
//	//	return errors.New(err, "set ipv4 dns")
//	//}
//	//err = luid.SetDNS(winipcfg.AddressFamily(windows.AF_INET6), []netip.Addr{t.inet6Address.Addr().Next()}, nil)
//	//if err != nil {
//	//	return errors.New(err, "set ipv6 dns")
//	//}
//	if t.autoRoute {
//		if t.inet4Address.IsValid() {
//			err := luid.AddRoute(netip.PrefixFrom(netip.IPv4Unspecified(), 0), netip.IPv4Unspecified(), 0)
//			if err != nil {
//				return errors.New(err, "set ipv4 route")
//			}
//		}
//		//if t.inet6Address.IsValid() {
//		//	err = luid.AddRoute(netip.PrefixFrom(netip.IPv6Unspecified(), 0), netip.IPv6Unspecified(), 0)
//		//	if err != nil {
//		//		return errors.New(err, "set ipv6 route")
//		//	}
//		//}
//	}
//	if t.inet4Address.IsValid() {
//		//var inetIf *winipcfg.MibIPInterfaceRow
//		inetIf, err := luid.IPInterface(winipcfg.AddressFamily(windows.AF_INET))
//		if err != nil {
//			return err
//		}
//		inetIf.ForwardingEnabled = true
//		inetIf.RouterDiscoveryBehavior = winipcfg.RouterDiscoveryDisabled
//		inetIf.DadTransmits = 0
//		inetIf.ManagedAddressConfigurationSupported = false
//		inetIf.OtherStatefulConfigurationSupported = false
//		inetIf.NLMTU = t.mtu
//		if t.autoRoute {
//			inetIf.UseAutomaticMetric = false
//			inetIf.Metric = 0
//		}
//		err = inetIf.Set()
//		if err != nil {
//			return errors.New(err, "set ipv4 options")
//		}
//	}
//	//if t.inet6Address.IsValid() {
//	//	var inet6If *winipcfg.MibIPInterfaceRow
//	//	inet6If, err = luid.IPInterface(winipcfg.AddressFamily(windows.AF_INET6))
//	//	if err != nil {
//	//		return err
//	//	}
//	//	inet6If.RouterDiscoveryBehavior = winipcfg.RouterDiscoveryDisabled
//	//	inet6If.DadTransmits = 0
//	//	inet6If.ManagedAddressConfigurationSupported = false
//	//	inet6If.OtherStatefulConfigurationSupported = false
//	//	inet6If.NLMTU = t.mtu
//	//	if t.autoRoute {
//	//		inet6If.UseAutomaticMetric = false
//	//		inet6If.Metric = 0
//	//	}
//	//	err = inet6If.Set()
//	//	if err != nil {
//	//		return errors.New(err, "set ipv6 options")
//	//	}
//	//}
//	return nil
//}

func (t *WinTun) configure() error {
	luid := winipcfg.LUID(t.adapter.LUID())
	luid.FlushIPAddresses(winipcfg.AddressFamily(windows.AF_INET))
	luid.FlushDNS(winipcfg.AddressFamily(windows.AF_INET))

	if t.inet4Address.IsValid() {
		err := luid.SetIPAddressesForFamily(winipcfg.AddressFamily(windows.AF_INET), []netip.Prefix{t.inet4Address})
		if err != nil {
			return errors.New(err, "set ipv4 address")
		}

		// Set DNS for IPv4
		err = luid.SetDNS(winipcfg.AddressFamily(windows.AF_INET), []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil)
		if err != nil {
			return errors.New(err, "set ipv4 dns")
		}
	}

	if t.autoRoute {
		if t.inet4Address.IsValid() {
			err := luid.AddRoute(netip.PrefixFrom(netip.IPv4Unspecified(), 0), netip.IPv4Unspecified(), 0)
			if err != nil {
				return errors.New(err, "set ipv4 route")
			}
		}
	}

	if t.inet4Address.IsValid() {
		inetIf, err := luid.IPInterface(winipcfg.AddressFamily(windows.AF_INET))
		if err != nil {
			return err
		}
		inetIf.ForwardingEnabled = true
		inetIf.RouterDiscoveryBehavior = winipcfg.RouterDiscoveryDisabled
		inetIf.DadTransmits = 0
		inetIf.ManagedAddressConfigurationSupported = false
		inetIf.OtherStatefulConfigurationSupported = false
		inetIf.NLMTU = t.mtu
		if t.autoRoute {
			inetIf.UseAutomaticMetric = false
			inetIf.Metric = 0
		}
		err = inetIf.Set()
		if err != nil {
			return errors.New(err, "set ipv4 options")
		}
	}

	return nil
}

func (t *WinTun) Read() ([]byte, error) {
	t.waitGroup.Add(1)
	defer t.waitGroup.Done()
	var n int
retry:
	if t.close.Load() == 1 {
		return nil, os.ErrClosed
	}
	start := nanotime()
	shouldSpin := t.rate.current.Load() >= spinloopRateThreshold && uint64(start-t.rate.nextStartTime.Load()) <= rateMeasurementGranularity*2
	for {
		if t.close.Load() == 1 {
			return nil, os.ErrClosed
		}

		packet, err := t.session.ReceivePacket()
		switch err {
		case nil:
			p := make([]byte, len(packet))
			copy(p, packet)
			t.session.ReleaseReceivePacket(packet)
			t.rate.update(uint64(n))
			return p, nil
		case windows.ERROR_NO_MORE_ITEMS:
			if !shouldSpin || uint64(nanotime()-start) >= spinloopDuration {
				windows.WaitForSingleObject(t.readWait, windows.INFINITE)
				goto retry
			}
			procyield(1)
			continue
		case windows.ERROR_HANDLE_EOF:
			return nil, os.ErrClosed
		case windows.ERROR_INVALID_DATA:
			return nil, errors.New("send ring corrupt")
		}
		return nil, fmt.Errorf("read failed: %w", err)
	}
}

func (t *WinTun) Write(p []byte) (n int, err error) {
	t.waitGroup.Add(1)
	defer t.waitGroup.Done()

	if t.close.Load() == 1 {
		return 0, os.ErrClosed
	}

	t.rate.update(uint64(len(p)))
	packet, err := t.session.AllocateSendPacket(len(p))
	copy(packet, p)
	if err == nil {
		t.session.SendPacket(packet)
		return len(p), nil
	}
	switch err {
	case windows.ERROR_HANDLE_EOF:
		return 0, os.ErrClosed
	case windows.ERROR_BUFFER_OVERFLOW:
		return 0, nil
	}
	return 0, fmt.Errorf("write failed: %w", err)
}

func (t *WinTun) NewEndpoint() (stack.LinkEndpoint, error) {
	return &WintunEndpoint{tun: t}, nil
}

func (t *WinTun) Close() error {
	t.closeOnce.Do(func() {
		t.close.Store(1)
		windows.SetEvent(t.readWait)
		t.waitGroup.Wait()
		t.session.End()
		t.adapter.Close()
	})

	return nil
}

func generateGUIDByDeviceName(name string) *windows.GUID {
	hash := sha256.New()
	hash.Write([]byte("xray-tun-"))
	hash.Write([]byte(name))
	sum := hash.Sum(nil)
	return (*windows.GUID)(unsafe.Pointer(&sum[0]))
}

//go:linkname procyield runtime.procyield
func procyield(cycles uint32)

//go:linkname nanotime runtime.nanotime
func nanotime() int64

type rateJuggler struct {
	current       atomic.Uint64
	nextByteCount atomic.Uint64
	nextStartTime atomic.Int64
	changing      atomic.Int32
}

func (rate *rateJuggler) update(packetLen uint64) {
	now := nanotime()
	total := rate.nextByteCount.Add(packetLen)
	period := uint64(now - rate.nextStartTime.Load())
	if period >= rateMeasurementGranularity {
		if !rate.changing.CompareAndSwap(0, 1) {
			return
		}
		rate.nextStartTime.Store(now)
		rate.current.Store(total * uint64(time.Second/time.Nanosecond) / period)
		rate.nextByteCount.Store(0)
		rate.changing.Store(0)
	}
}

const (
	rateMeasurementGranularity = uint64((time.Second / 2) / time.Nanosecond)
	spinloopRateThreshold      = 800000000 / 8                                   // 800mbps
	spinloopDuration           = uint64(time.Millisecond / 80 / time.Nanosecond) // ~1gbit/s
)
