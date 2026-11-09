package importer

import (
	"errors"
	"strings"

	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/agentclient"
)

type Backend struct {
	Kind       string
	EndpointID string
	Client     agentclient.Gateway
}
type Routing struct {
	Current      Backend
	LegacyDirect Backend
	Retained     []Backend
}

func NewRouted(dir string, r Routing) (*Service, error) {
	if (r.Current.Kind != "direct" && r.Current.Kind != "platform") || r.Current.EndpointID == "" || r.LegacyDirect.Kind != "direct" || r.LegacyDirect.EndpointID == "" {
		return nil, errors.New("invalid import backend routing")
	}
	s := New(dir, nil)
	s.current = r.Current
	s.legacy = r.LegacyDirect
	s.routes = map[string]Backend{}
	for _, b := range append([]Backend{r.LegacyDirect, r.Current}, r.Retained...) {
		if (b.Kind != "direct" && b.Kind != "platform") || b.EndpointID == "" || len(b.EndpointID) > 128 || strings.ContainsAny(b.EndpointID, "/\\\x00") {
			return nil, errors.New("invalid retained import endpoint")
		}
		if _, exists := s.routes[b.Kind+":"+b.EndpointID]; !exists {
			s.routes[b.Kind+":"+b.EndpointID] = b
		}
	}
	return s, nil
}
func (s *Service) backend(d *Draft) (agentclient.Gateway, error) {
	if s.routes == nil {
		if s.agent == nil {
			return nil, errors.New("AI接続が未設定です。下書きは保持されています")
		}
		return s.agent, nil
	}
	kind, id := d.BackendKind, d.BackendEndpoint
	if kind == "" && id == "" {
		kind, id = s.legacy.Kind, s.legacy.EndpointID
	}
	b, ok := s.routes[kind+":"+id]
	if !ok || b.Client == nil {
		return nil, errors.New("元のAI接続を利用できません。下書きは保持されています")
	}
	return b.Client, nil
}
func (s *Service) pinBackend(d *Draft) {
	if s.routes == nil {
		return
	}
	if d.BackendKind == "" && d.BackendEndpoint == "" {
		d.BackendKind = s.legacy.Kind
		d.BackendEndpoint = s.legacy.EndpointID
	}
}
func (s *Service) backendWaiting(d *Draft) {
	d.AgentAvailable = false
	warning := "元のAI接続を利用できません。原文と下書きは保持されています。手入力に切り替えるか、接続設定の復旧後に再確認してください。"
	if d.Job != nil && (d.Job.State == "queued" || d.Job.State == "running" || d.Job.State == "waiting") {
		d.Job.State = "waiting"
		d.Job.Reason = "backend_unavailable"
	}
	for _, w := range d.Warnings {
		if w == warning {
			return
		}
	}
	d.Warnings = append(d.Warnings, warning)
}
