package main

import (
	"agent-router/internal/upstream"
)

type Upstream = upstream.Upstream
type LoadBalancer = upstream.LoadBalancer
type SharedUpstreams = upstream.SharedUpstreams

var NewLoadBalancer = upstream.NewLoadBalancer
var NewSharedUpstreams = upstream.NewSharedUpstreams
