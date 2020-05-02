package main

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

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
	p := &Plugins{
		mgr:       subrpc.NewManager(),
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
		err := p.mgr.NewProcess(subrpc.ProcessOptions{
			Name:    i.Name,
			Type:    i.Type,
			Config:  i.Config,
			ExePath: "./plugins.d/" + i.ExeName,
			Env:     i.Env,
			Token:   TRUSTTOKEN,
		})
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Start function
func (p *Plugins) Start() {
	errs := p.mgr.StartAllProcess()
	if len(errs) > 0 {
		for _, e := range errs {
			p.log.Error(e)
			return
		}
	}
	go p.Logger()
	<-p.terminate
	errs = p.mgr.StopAll()
	if len(errs) > 0 {
		for _, e := range errs {
			p.log.Error(e)
			return
		}
	}
	return
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
	return map[string]interface{}{}
}

//Logger function
func (p *Plugins) Logger() {
	t := time.NewTicker(200 * time.Millisecond)
	for range t.C {
		l, err := p.mgr.OutBuffer.ReadString('\n')
		if err != nil && err != io.EOF {
			p.log.Error(err)
		}
		if l != "" {
			p.log.Debug(l)
		}
		l, err = p.mgr.ErrBuffer.ReadString('\n')
		if err != nil && err != io.EOF {
			p.log.Error(err)
		}
		if l != "" {
			p.log.Warn(l)
		}
	}
}
