package health

import (
	"github.com/k0kubun/pp"
	"github.com/rai-project/config"
	"github.com/rai-project/vipertags"
)

type healthConfig struct {
	Endpoints []string      `json:"endpoints" config:"health.endpoints"`
	Metrics   []string      `json:"metrics" config:"health.metrics"`
	done      chan struct{} `json:"-" config:"-"`
}

var (
	Config = &healthConfig{
		done: make(chan struct{}),
	}
)

func (healthConfig) ConfigName() string {
	return "Health"
}

func (a *healthConfig) SetDefaults() {
	vipertags.SetDefaults(a)
}

func (a *healthConfig) Read() {
	defer close(a.done)
	vipertags.Fill(a)
}

func (c healthConfig) Wait() {
	<-c.done
}

func (c healthConfig) String() string {
	return pp.Sprintln(c)
}

func (c healthConfig) Debug() {
	log.Debug("Health Config = ", c)
}

func init() {
	config.Register(Config)
}
