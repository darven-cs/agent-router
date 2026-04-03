package main

import (
	"hash/fnv"
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
