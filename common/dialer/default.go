package dialer

import (
	"context"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/control"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ WireGuardListener = (*DefaultDialer)(nil)

type DefaultDialer struct {
	dialer4             tcpDialer
	dialer6             tcpDialer
	udpDialer4          net.Dialer
	udpDialer6          net.Dialer
	udpListener         net.ListenConfig
	udpAddr4            string
	udpAddr6            string
	isWireGuardListener bool
}

func NewDefault(router adapter.Router, options option.DialerOptions) (*DefaultDialer, error) {
	var dialer net.Dialer
	var listener net.ListenConfig
	if options.BindInterface != "" {
		bindFunc := control.BindToInterface(router.InterfaceFinder(), options.BindInterface, -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	} else if router.AutoDetectInterface() {
		bindFunc := router.AutoDetectInterfaceFunc()
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	} else if router.DefaultInterface() != "" {
		bindFunc := control.BindToInterface(router.InterfaceFinder(), router.DefaultInterface(), -1)
		dialer.Control = control.Append(dialer.Control, bindFunc)
		listener.Control = control.Append(listener.Control, bindFunc)
	}
	if options.RoutingMark != 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(options.RoutingMark))
		listener.Control = control.Append(listener.Control, control.RoutingMark(options.RoutingMark))
	} else if router.DefaultMark() != 0 {
		dialer.Control = control.Append(dialer.Control, control.RoutingMark(router.DefaultMark()))
		listener.Control = control.Append(listener.Control, control.RoutingMark(router.DefaultMark()))
	}
	if options.ReuseAddr {
		listener.Control = control.Append(listener.Control, control.ReuseAddr())
	}
	if options.ProtectPath != "" {
		dialer.Control = control.Append(dialer.Control, control.ProtectPath(options.ProtectPath))
		listener.Control = control.Append(listener.Control, control.ProtectPath(options.ProtectPath))
	}
	if options.ConnectTimeout != 0 {
		dialer.Timeout = time.Duration(options.ConnectTimeout)
	} else {
		dialer.Timeout = C.TCPTimeout
	}
	var udpFragment bool
	if options.UDPFragment != nil {
		udpFragment = *options.UDPFragment
	} else {
		udpFragment = options.UDPFragmentDefault
	}
	if !udpFragment {
		dialer.Control = control.Append(dialer.Control, control.DisableUDPFragment())
		listener.Control = control.Append(listener.Control, control.DisableUDPFragment())
	}
	var (
		dialer4    = dialer
		udpDialer4 = dialer
		udpAddr4   string
	)
	if options.Inet4BindAddress != nil {
		bindAddr := options.Inet4BindAddress.Build()
		dialer4.LocalAddr = &net.TCPAddr{IP: bindAddr.AsSlice()}
		udpDialer4.LocalAddr = &net.UDPAddr{IP: bindAddr.AsSlice()}
		udpAddr4 = M.SocksaddrFrom(bindAddr, 0).String()
	}
	var (
		dialer6    = dialer
		udpDialer6 = dialer
		udpAddr6   string
	)
	if options.Inet6BindAddress != nil {
		bindAddr := options.Inet6BindAddress.Build()
		dialer6.LocalAddr = &net.TCPAddr{IP: bindAddr.AsSlice()}
		udpDialer6.LocalAddr = &net.UDPAddr{IP: bindAddr.AsSlice()}
		udpAddr6 = M.SocksaddrFrom(bindAddr, 0).String()
	}
	if options.TCPMultiPath {
		if !go121Available {
			return nil, E.New("MultiPath TCP requires go1.21, please recompile your binary.")
		}
		setMultiPathTCP(&dialer4)
	}
	if options.IsWireGuardListener {
		for _, controlFn := range wgControlFns {
			listener.Control = control.Append(listener.Control, controlFn)
		}
	}
	tcpDialer4, err := newTCPDialer(dialer4, options.TCPFastOpen)
	if err != nil {
		return nil, err
	}
	tcpDialer6, err := newTCPDialer(dialer6, options.TCPFastOpen)
	if err != nil {
		return nil, err
	}
	return &DefaultDialer{
		tcpDialer4,
		tcpDialer6,
		udpDialer4,
		udpDialer6,
		listener,
		udpAddr4,
		udpAddr6,
		options.IsWireGuardListener,
	}, nil
}

type TCPConn struct {
	conn net.Conn
	err  error
}

func GetTCPConn(connChan chan TCPConn) TCPConn {
	var tcpConn TCPConn
	for i := 0; i < 3; i++ {
		tcpConn = <-connChan
		if tcpConn.err != nil {
			continue
		}
		go func(index int) {
			for i := index; i < 3; i++ {
				tcpConn := <-connChan
				if tcpConn.err != nil {
					continue
				}
				tcpConn.conn.Close()
			}
		}(i + 1)
		break
	}
	return tcpConn
}

func (d *DefaultDialer) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	if !address.IsValid() {
		return nil, E.New("invalid address")
	}
	connChan := make(chan TCPConn, 3)
	for i := 0; i < 3; i++ {
		go func() {
			var tcpConn TCPConn
			for i := 0; i < 4; i++ {
				if tcpConn.conn, tcpConn.err = func() (net.Conn, error) {
					if N.NetworkName(network) == N.NetworkUDP {
						if !address.IsIPv6() {
							return d.udpDialer4.DialContext(ctx, network, address.String())
						}
						return d.udpDialer6.DialContext(ctx, network, address.String())
					} else if !address.IsIPv6() {
						return DialSlowContext(&d.dialer4, ctx, network, address)
					}
					return DialSlowContext(&d.dialer6, ctx, network, address)
				}(); tcpConn.err == nil {
					break
				}
			}
			connChan <- tcpConn
		}()
	}
	tcpConn := GetTCPConn(connChan)
	return trackConn(tcpConn.conn, tcpConn.err)
}

type UDPConn struct {
	conn net.PacketConn
	err  error
}

func GetUDPConn(connChan chan UDPConn) UDPConn {
	var udpConn UDPConn
	for i := 0; i < 3; i++ {
		udpConn = <-connChan
		if udpConn.err != nil {
			continue
		}
		go func(index int) {
			for i := index; i < 3; i++ {
				udpConn := <-connChan
				if udpConn.err != nil {
					continue
				}
				udpConn.conn.Close()
			}
		}(i + 1)
		break
	}
	return udpConn
}

func (d *DefaultDialer) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	connChan := make(chan UDPConn, 3)
	for i := 0; i < 3; i++ {
		go func() {
			var udpConn UDPConn
			for i := 0; i < 4; i++ {
				if udpConn.conn, udpConn.err = func() (net.PacketConn, error) {
					if destination.IsIPv6() {
						return d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr6)
					} else if destination.IsIPv4() && !destination.Addr.IsUnspecified() {
						return d.udpListener.ListenPacket(ctx, N.NetworkUDP+"4", d.udpAddr4)
					}
					return d.udpListener.ListenPacket(ctx, N.NetworkUDP, d.udpAddr4)
				}(); udpConn.err == nil {
					break
				}
			}
			connChan <- udpConn
		}()
	}
	udpConn := GetUDPConn(connChan)
	return trackPacketConn(udpConn.conn, udpConn.err)
}

func (d *DefaultDialer) ListenPacketCompat(network, address string) (net.PacketConn, error) {
	connChan := make(chan UDPConn, 3)
	for i := 0; i < 3; i++ {
		go func() {
			var udpConn UDPConn
			for i := 0; i < 4; i++ {
				if udpConn.conn, udpConn.err = d.udpListener.ListenPacket(context.Background(), network, address); udpConn.err == nil {
					break
				}
			}
			connChan <- udpConn
		}()
	}
	udpConn := GetUDPConn(connChan)
	return trackPacketConn(udpConn.conn, udpConn.err)
}

func trackConn(conn net.Conn, err error) (net.Conn, error) {
	if !conntrack.Enabled || err != nil {
		return conn, err
	}
	return conntrack.NewConn(conn)
}

func trackPacketConn(conn net.PacketConn, err error) (net.PacketConn, error) {
	if !conntrack.Enabled || err != nil {
		return conn, err
	}
	return conntrack.NewPacketConn(conn)
}
