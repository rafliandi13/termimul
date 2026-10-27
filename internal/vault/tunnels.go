package vault

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

// Tunnel is a saved port-forwarding rule that runs over a host's SSH connection.
//
//   - local   (-L): listen on BindAddress:BindPort here, connect to TargetHost:TargetPort from the server
//   - remote  (-R): listen on BindAddress:BindPort on the server, connect to TargetHost:TargetPort from here
//   - dynamic (-D): SOCKS5 proxy on BindAddress:BindPort here; the server makes the connections
type Tunnel struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	HostID      string `json:"hostId"`
	Type        string `json:"type"` // "local", "remote" or "dynamic"
	BindAddress string `json:"bindAddress"`
	BindPort    int    `json:"bindPort"`
	TargetHost  string `json:"targetHost"`
	TargetPort  int    `json:"targetPort"`
	AutoStart   bool   `json:"autoStart"`
}

func (s *Store) Tunnels() []Tunnel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Tunnel{}, s.data.Tunnels...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label) })
	return out
}

func (s *Store) Tunnel(id string) (Tunnel, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.data.Tunnels {
		if t.ID == id {
			return t, true
		}
	}
	return Tunnel{}, false
}

func (s *Store) SaveTunnel(t Tunnel) (Tunnel, error) {
	if err := normalizeTunnel(&t); err != nil {
		return Tunnel{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hostExistsLocked(t.HostID) {
		return Tunnel{}, errors.New("pilih host untuk tunnel ini")
	}
	for i, existing := range s.data.Tunnels {
		if existing.ID == t.ID {
			s.data.Tunnels[i] = t
			return t, s.flush()
		}
	}
	t.ID = newID()
	s.data.Tunnels = append(s.data.Tunnels, t)
	return t, s.flush()
}

func (s *Store) DeleteTunnel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.data.Tunnels {
		if t.ID == id {
			s.data.Tunnels = append(s.data.Tunnels[:i], s.data.Tunnels[i+1:]...)
			return s.flush()
		}
	}
	return nil
}

func (s *Store) hostExistsLocked(id string) bool {
	for _, h := range s.data.Hosts {
		if h.ID == id {
			return true
		}
	}
	return false
}

func normalizeTunnel(t *Tunnel) error {
	t.Label = strings.TrimSpace(t.Label)
	t.BindAddress = strings.TrimSpace(t.BindAddress)
	t.TargetHost = strings.TrimSpace(t.TargetHost)
	switch t.Type {
	case "local", "remote", "dynamic":
	default:
		return errors.New("jenis tunnel harus local, remote, atau dynamic")
	}
	if t.BindPort < 1 || t.BindPort > 65535 {
		return errors.New("port listen harus antara 1 dan 65535")
	}
	// Loopback by default: exposing a tunnel to the network has to be a deliberate choice.
	if t.BindAddress == "" {
		t.BindAddress = "127.0.0.1"
	}
	if t.Type == "dynamic" {
		t.TargetHost, t.TargetPort = "", 0
	} else {
		if t.TargetHost == "" {
			return errors.New("host tujuan wajib diisi")
		}
		if t.TargetPort < 1 || t.TargetPort > 65535 {
			return errors.New("port tujuan harus antara 1 dan 65535")
		}
	}
	if t.Label == "" {
		t.Label = describeTunnel(*t)
	}
	return nil
}

func describeTunnel(t Tunnel) string {
	switch t.Type {
	case "dynamic":
		return "SOCKS " + strconv.Itoa(t.BindPort)
	case "remote":
		return "Remote " + strconv.Itoa(t.BindPort) + " → " + t.TargetHost + ":" + strconv.Itoa(t.TargetPort)
	}
	return "Local " + strconv.Itoa(t.BindPort) + " → " + t.TargetHost + ":" + strconv.Itoa(t.TargetPort)
}
