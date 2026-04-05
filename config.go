package main

import "agent-router/internal/config"

type ServiceConfig = config.ServiceConfig
type UpstreamConfig = config.UpstreamConfig
type Config = config.Config

var LoadConfig = config.LoadConfig
var SaveConfig = config.SaveConfig
