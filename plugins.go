package main

import (
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	subrpc "github.com/yeticloud/libsubrpc"
	"gopkg.in/yaml.v2"
)

//Plugins type
type Plugins struct {
	mgr       *subrpc.Manager
	app       *Cave
	terminate chan bool
	config    *Config
	log       *Log
	metrics   map[string]interface{}
}

// NewPlugins function
func NewPlugins(app *Cave) (*Plugins, error) {
	mgr, err := subrpc.NewManager()
	if err != nil {
		return nil, err
	}
	p := &Plugins{
		mgr:       mgr,
		app:       app,
		terminate: make(chan bool),
		config:    app.Config,
		log:       app.Logger,
		metrics:   pluginMetrics(),
	}
	plugs, err := pluginList("./plugins.d")
	if err != nil {
		return nil, err
	}
	for _, i := range plugs {
		t, err := app.TokenStore.Issue(i.Name, "plugin:"+i.Type, false)
		if err != nil {
			return nil, err
		}
		err = p.mgr.NewProcess(subrpc.ProcessOptions{
			Name:         i.Name,
			Type:         i.Type,
			Config:       i.Config,
			ExePath:      "./plugins.d/" + i.ExeName,
			Env:          i.Env,
			Token:        t.Token,
			StartupDelay: time.Duration(i.StartupDelay) * time.Second,
		})
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Start function
func (p *Plugins) Start() {
	p.log.Debug("PLUGIN", "Starting plugins")
	errs := p.mgr.StartAllProcess()
	if len(errs) > 0 {
		for _, e := range errs {
			p.log.Error("PLUGIN", e)
		}
	}
	go p.Logger()
	go p.doMetrics()
	p.log.Debug("PLUGIN", "Done starting plugins")
	_ = <-p.terminate
	errs = p.mgr.StopAll()
	if len(errs) > 0 {
		for _, e := range errs {
			p.log.Error("PLUGIN", e)
			return
		}
	}

	return
}

func (p *Plugins) doMetrics() {
	t := time.NewTicker(1 * time.Second)
	for range t.C {
		count := 0
		running := 0
		stopped := 0
		for t, v := range p.mgr.Procs {
			count = count + len(v)
			p.metrics["plugin_by_type"].(*prometheus.GaugeVec).WithLabelValues(t).Set(float64(count))
			for n, i := range v {
				p.metrics["plugin_details"].(*prometheus.GaugeVec).WithLabelValues(n, t, i.Options.ExePath, strconv.FormatBool(i.Running)).Set(1.0)
				if i.Running {
					running++
				} else {
					stopped++
				}
			}
		}
		p.metrics["plugin_count_total"].(prometheus.Gauge).Set(float64(count))
		p.metrics["plugin_count_running"].(prometheus.Gauge).Set(float64(running))
		p.metrics["plugin_count_stopped"].(prometheus.Gauge).Set(float64(stopped))
		for len(p.mgr.Metrics) > 0 {
			met := <-p.mgr.Metrics
			urn := strings.Split(met.URN, ":")
			if len(urn) != 3 {
				continue
			}
			p.metrics["plugin_call_time_by_urn"].(*prometheus.GaugeVec).WithLabelValues(urn[0], urn[1], urn[2], strconv.FormatBool(met.Error)).Set(float64(met.CallTime.Milliseconds()))
			p.metrics["plugin_call_count_by_urn"].(*prometheus.GaugeVec).WithLabelValues(urn[0], urn[1], urn[2], strconv.FormatBool(met.Error)).Inc()
		}
	}
}

//Logger function
func (p *Plugins) Logger() {
	t := time.NewTicker(200 * time.Millisecond)
	for range t.C {
		l, err := p.mgr.OutBuffer.ReadString('\n')
		if err != nil && err != io.EOF {
			p.log.Error("PLUGIN", err)
		}
		if l != "" {
			p.log.Debug("PLUGIN", l)
		}
		l, err = p.mgr.ErrBuffer.ReadString('\n')
		if err != nil && err != io.EOF {
			p.log.Error("PLUGIN", err)
		}
		if l != "" {
			p.log.Warn("PLUGIN", l)
		}
	}
}

func pluginList(path string) (map[string]PluginConfig, error) {
	res := map[string]PluginConfig{}
	ls, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	yamls := []string{}
	for _, f := range ls {
		if strings.HasSuffix(f.Name(), ".yaml") || strings.HasSuffix(f.Name(), ".yml") {
			yamls = append(yamls, f.Name())
		}
	}
	for _, y := range yamls {
		var c PluginConfig
		f, err := ioutil.ReadFile(path + "/" + y)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(f, &c)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(path + "/" + c.ExeName); err == nil {
			res[c.Name] = c
		}
	}
	return res, nil
}

func pluginMetrics() map[string]interface{} {
	return map[string]interface{}{
		"plugin_count_total": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cave_plugin_count_total",
			Help: "The current number of total plugins",
		}),
		"plugin_count_running": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cave_plugin_count_running",
			Help: "The current number of running plugins",
		}),
		"plugin_count_stopped": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cave_plugin_count_stopped",
			Help: "The current number of stopped plugins",
		}),
		"plugin_by_type": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_plugin_count_types",
			Help: "The current number of plugins by type",
		}, []string{"type"}),
		"plugin_details": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_plugin_details",
			Help: "The current running plugins details",
		}, []string{"name", "type", "exe_path", "running"}),
		"plugin_call_count": promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cave_plugin_call_count",
			Help: "The number of calls by plugin",
		}, []string{"name"}),
		"plugin_call_times": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_plugin_call_times",
			Help: "Call times by plugin name",
		}, []string{"name"}),
		"plugin_call_time_by_urn": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_plugin_call_time_by_urn",
			Help: "Call times by URN",
		}, []string{"type", "name", "function", "error"}),
		"plugin_call_count_by_urn": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cave_plugin_call_count_by_urn",
			Help: "Call count by URN",
		}, []string{"type", "name", "function", "error"}),
	}
}

// LOGGING IS NOT WORKING
