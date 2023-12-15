package outbound

import (
	"context"
	"net"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/interrupt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.Outbound      = (*Relay)(nil)
	_ adapter.OutboundGroup = (*Relay)(nil)
	_ adapter.RelayGroup    = (*Relay)(nil)
)

type Relay struct {
	myOutboundAdapter
	tags                         []string
	interruptGroup               *interrupt.Group
	interruptExternalConnections bool
}

func NewRelay(router adapter.Router, logger log.ContextLogger, tag string, options option.RelayOutboundOptions) (*Relay, error) {
	outbound := &Relay{
		myOutboundAdapter: myOutboundAdapter{
			protocol:     C.TypeRelay,
			router:       router,
			logger:       logger,
			tag:          tag,
			dependencies: options.Outbounds,
		},
		tags:                         options.Outbounds,
		interruptGroup:               interrupt.NewGroup(),
		interruptExternalConnections: options.InterruptExistConnections,
	}
	if len(outbound.tags) == 0 {
		return nil, E.New("missing tags")
	}
	return outbound, nil
}

func (r *Relay) Network() []string {
	detour, _ := r.router.Outbound(r.tags[0])
	return detour.Network()
}

func (r *Relay) Start() error {
	for i, tag := range r.tags {
		outbound, loaded := r.router.Outbound(tag)
		if !loaded {
			return E.New("outbound ", i, " not found: ", tag)
		}
		if _, isRelay := outbound.(adapter.RelayGroup); isRelay {
			return E.New("relay outbound invalid: ", tag)
		}
	}
	return nil
}

func (r *Relay) Now() string {
	return ""
}

func (r *Relay) All() []string {
	return r.tags
}

func (s *Relay) UpdateOutbounds(tag string) error {
	return nil
}

func (s *Relay) IsRelay() bool {
	return true
}

func (r *Relay) SelectedOutbound(network string) adapter.Outbound {
	detour, _ := r.router.Outbound(r.tags[len(r.tags)-1])
	return detour
}

func (r *Relay) createRelayChain(network string) adapter.Outbound {
	len := len(r.tags)
	detour, _ := r.router.Outbound(r.tags[0])
	tag := RealOutboundTag(detour, network)
	detour, _ = r.router.Outbound(tag)
	for i := 1; i < len; i++ {
		out, _ := r.router.Outbound(r.tags[i])
		tag := RealOutboundTag(out, network)
		out, _ = r.router.Outbound(tag)
		outbound := out.(adapter.OutboundRelay)
		d := dialer.NewDetourWithDialer(r.router, detour)
		detour = outbound.SetRelay(d)
	}
	return detour
}

func (r *Relay) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	detour := r.createRelayChain(network)
	conn, err := detour.DialContext(ctx, network, destination)
	if err != nil {
		return nil, err
	}
	return r.interruptGroup.NewConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (r *Relay) ListenPacket(ctx context.Context, destination M.Socksaddr) (net.PacketConn, error) {
	detour := r.createRelayChain(N.NetworkUDP)
	conn, err := detour.ListenPacket(ctx, destination)
	if err != nil {
		return nil, err
	}
	return r.interruptGroup.NewPacketConn(conn, interrupt.IsExternalConnectionFromContext(ctx)), nil
}

func (r *Relay) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	detour := r.createRelayChain(metadata.Network)
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	return detour.NewConnection(ctx, conn, metadata)
}

func (r *Relay) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	detour := r.createRelayChain(metadata.Network)
	ctx = interrupt.ContextWithIsExternalConnection(ctx)
	return detour.NewPacketConnection(ctx, conn, metadata)
}
