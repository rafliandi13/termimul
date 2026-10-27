package services

import (
	"errors"

	"termimul/internal/tunnel"
	"termimul/internal/vault"
)

// TunnelService manages port-forwarding rules. Status changes are pushed as
// "tunnel:status" events carrying a tunnel.Status.
type TunnelService struct{ *env }

type TunnelView struct {
	vault.Tunnel
	Status tunnel.Status `json:"status"`
}

func (s *TunnelService) List() []TunnelView {
	rules := s.store.Tunnels()
	out := make([]TunnelView, 0, len(rules))
	for _, t := range rules {
		out = append(out, TunnelView{Tunnel: t, Status: s.tunnels.Status(t.ID)})
	}
	return out
}

// Save stores a rule. A running tunnel keeps its old settings until restarted.
func (s *TunnelService) Save(t vault.Tunnel) (vault.Tunnel, error) {
	return s.store.SaveTunnel(t)
}

func (s *TunnelService) Delete(id string) error {
	s.tunnels.Forget(id)
	return s.store.DeleteTunnel(id)
}

// Start connects to the tunnel's host and begins forwarding. Errors are also
// reported through the tunnel's status.
func (s *TunnelService) Start(id string) error {
	t, ok := s.store.Tunnel(id)
	if !ok {
		return errors.New("tunnel tidak ditemukan")
	}
	s.tunnels.SetStarting(id)
	client, err := s.dialer.Dial(s.ctx, t.HostID)
	if err != nil {
		s.tunnels.Fail(id, err)
		return err
	}
	return s.tunnels.Start(t, client)
}

func (s *TunnelService) Stop(id string) {
	s.tunnels.Stop(id)
}

// startAutomatic starts every rule marked AutoStart, in the background so a
// slow or unreachable server doesn't delay the window.
func (s *TunnelService) startAutomatic() {
	for _, t := range s.store.Tunnels() {
		if t.AutoStart {
			go s.Start(t.ID)
		}
	}
}
