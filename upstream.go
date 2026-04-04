package main

import (
	"hash/fnv"
	"sync"
	"time"
)

// Upstream represents a single upstream provider
type Upstream struct {
	Name     string
	URL      string
	APIKey   string
	AuthType string // "bearer" or "x-api-key"
	Enabled  bool
	Timeout  time.Duration
	Model    string // upstream-specific model name (if empty, use request model)
}

// LoadBalancer distributes requests across enabled upstreams using modulo hash
type LoadBalancer struct {
	upstreams []*Upstream
}

// NewLoadBalancer creates a load balancer from upstream configurations
func NewLoadBalancer(configs []UpstreamConfig) LoadBalancer {
	var upstreams []*Upstream
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		upstreams = append(upstreams, &Upstream{
			Name:     cfg.Name,
			URL:      cfg.URL,
			APIKey:   cfg.APIKey,
			AuthType: cfg.AuthType,
			Enabled:  cfg.Enabled,
			Timeout:  time.Duration(cfg.Timeout) * time.Second,
			Model:    cfg.Model,
		})
	}
	return LoadBalancer{upstreams: upstreams}
}

// Select chooses an upstream using FNV-1a hash modulo
func (lb LoadBalancer) Select(hashInput string) *Upstream {
	if len(lb.upstreams) == 0 {
		return nil
	}
	h := fnv.New32a()
	h.Write([]byte(hashInput))
	hash := h.Sum32()
	index := hash % uint32(len(lb.upstreams))
	return lb.upstreams[index]
}

// GetEnabled returns all enabled upstreams
func (lb LoadBalancer) GetEnabled() []*Upstream {
	return lb.upstreams
}

// SelectNext returns the next upstream after 'after', wrapping to first if at end.
// Returns nil if no upstreams available.
func (lb LoadBalancer) SelectNext(after *Upstream) *Upstream {
	if len(lb.upstreams) == 0 {
		return nil
	}
	if after == nil {
		return lb.upstreams[0]
	}
	for i, us := range lb.upstreams {
		if us == after {
			// Return next, wrapping to 0 if at end
			next := (i + 1) % len(lb.upstreams)
			return lb.upstreams[next]
		}
	}
	return lb.upstreams[0] // 'after' not found, start from beginning
}

// SharedUpstreams is thread-safe shared state for upstreams that both
// TUI and ProxyHandler reference. Protected by RWMutex.
type SharedUpstreams struct {
	mu        sync.RWMutex
	upstreams []*Upstream
}

// NewSharedUpstreams creates a new SharedUpstreams instance
func NewSharedUpstreams(upstreams []*Upstream) *SharedUpstreams {
	return &SharedUpstreams{upstreams: upstreams}
}

// GetAll returns a copy of all upstreams
func (s *SharedUpstreams) GetAll() []*Upstream {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Upstream, len(s.upstreams))
	copy(result, s.upstreams)
	return result
}

// Add adds a new upstream
func (s *SharedUpstreams) Add(u *Upstream) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upstreams = append(s.upstreams, u)
}

// Update updates an existing upstream by name
func (s *SharedUpstreams) Update(name string, u *Upstream) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, us := range s.upstreams {
		if us.Name == name {
			s.upstreams[i] = u
			return true
		}
	}
	return false
}

// Delete removes an upstream by name
func (s *SharedUpstreams) Delete(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, us := range s.upstreams {
		if us.Name == name {
			s.upstreams = append(s.upstreams[:i], s.upstreams[i+1:]...)
			return true
		}
	}
	return false
}

// AddUpstream adds an upstream to the load balancer
func (lb *LoadBalancer) AddUpstream(u *Upstream) {
	lb.upstreams = append(lb.upstreams, u)
}

// UpdateUpstream updates an existing upstream in the load balancer
func (lb *LoadBalancer) UpdateUpstream(u *Upstream) {
	for i, us := range lb.upstreams {
		if us.Name == u.Name {
			lb.upstreams[i] = u
			return
		}
	}
}

// DeleteUpstream removes an upstream from the load balancer
func (lb *LoadBalancer) DeleteUpstream(name string) {
	for i, us := range lb.upstreams {
		if us.Name == name {
			lb.upstreams = append(lb.upstreams[:i], lb.upstreams[i+1:]...)
			return
		}
	}
}
