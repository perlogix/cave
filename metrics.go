package main

import (
	"os"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shirou/gopsutil/process"
)

func mainMetrics() {
	m := map[string]interface{}{
		// RUNTIME
		"numcpu": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_cpu_count",
			Help: "The number of CPU's the app runtime can see",
		}),
		"numgo": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_goroutine_count",
			Help: "The number of running goroutines",
		}),
		"lookups": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_lookups",
			Help: "Number of pointer lookups performed by the runtime",
		}),
		"mallocs": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_allocs",
			Help: "Cumulative count of heap objects allocated",
		}),
		"frees": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_frees",
			Help: "Cumulative count of freed heap objects",
		}),
		"heap_alloc": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_used_bytes",
			Help: "Total bytes used by the heap",
		}),
		"heap_total": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_total_bytes",
			Help: "Total bytes available for the heap",
		}),
		"heap_idle": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_idle_bytes",
			Help: "Bytes in idle (unused) spans",
		}),
		"heap_inuse": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_inuse_bytes",
			Help: "Bytes in in-use spans",
		}),
		"heap_released": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_released_bytes",
			Help: "Bytes of memory returned to the OS",
		}),
		"heap_objects": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_heap_objects_count",
			Help: "Number of objects in the heap",
		}),
		"gc_num": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_gc_count",
			Help: "Number of times the garbage collector has run",
		}),
		"gc_pause": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_runtime_gc_pause_ms",
			Help: "Number of miliseconds the garbage collector has paused the program",
		}), // PSUTIL
		"proc_cpu": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_cpu_pct",
			Help: "The percent of CPU that the bunker process uses",
		}),
		"proc_children": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_child_count",
			Help: "The number of child processes of the main PID",
		}),
		"net_conns": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_network_connection_count",
			Help: "The number of network connections the process owns",
		}),
		"proc_uptime": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_uptime",
			Help: "Process uptime",
		}),
		"proc_io": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bunker_process_io_bytes",
			Help: "Process IO in bytes",
		}, []string{"op"}),
		"proc_mempct": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_mem_pct",
			Help: "Process memory usage as pct of total system mem",
		}),
		"proc_net_io": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bunker_process_network_io_bytes",
			Help: "Process network IO in bytes",
		}, []string{"op"}),
		"proc_ctx": promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bunker_process_ctx_switch_count",
			Help: "Process context switches",
		}, []string{"type"}),
		"proc_fd": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_fd_count",
			Help: "Number of fd's the process has open",
		}),
		"proc_threads": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_thread_count",
			Help: "Number of threads associated with the process",
		}),
		"proc_files": promauto.NewGauge(prometheus.GaugeOpts{
			Name: "bunker_process_open_files_count",
			Help: "Number of open files assocaited with the process",
		}),
	}
	// IMPLEMENT!

	for {
		var memstats runtime.MemStats
		runtime.ReadMemStats(&memstats)
		pausems := time.Nanosecond * time.Duration(memstats.PauseTotalNs)
		m["numcpu"].(prometheus.Gauge).Set(float64(runtime.GOMAXPROCS(0)))
		m["numgo"].(prometheus.Gauge).Set(float64(runtime.NumGoroutine()))
		m["lookups"].(prometheus.Gauge).Set(float64(memstats.Lookups))
		m["mallocs"].(prometheus.Gauge).Set(float64(memstats.Mallocs))
		m["frees"].(prometheus.Gauge).Set(float64(memstats.Frees))
		m["heap_alloc"].(prometheus.Gauge).Set(float64(memstats.HeapAlloc))
		m["heap_total"].(prometheus.Gauge).Set(float64(memstats.HeapSys))
		m["heap_idle"].(prometheus.Gauge).Set(float64(memstats.HeapIdle))
		m["heap_inuse"].(prometheus.Gauge).Set(float64(memstats.HeapInuse))
		m["heap_released"].(prometheus.Gauge).Set(float64(memstats.HeapReleased))
		m["heap_objects"].(prometheus.Gauge).Set(float64(memstats.HeapObjects))
		m["gc_num"].(prometheus.Gauge).Set(float64(memstats.NumGC))
		m["gc_pause"].(prometheus.Gauge).Set(float64(pausems.Milliseconds()))

		p, err := process.NewProcess(int32(os.Getpid()))
		if err != nil {

			continue
		}
		cpu, err := p.CPUPercent()
		check(err)
		m["proc_cpu"].(prometheus.Gauge).Set(float64(cpu))
		children, err := p.Children()
		check(err)
		m["proc_children"].(prometheus.Gauge).Set(float64(len(children)))
		netconns, err := p.Connections()
		check(err)
		m["net_conns"].(prometheus.Gauge).Set(float64(len(netconns)))
		stime, err := p.CreateTime()
		check(err)
		uptime := (time.Millisecond * time.Duration(stime)).Seconds()
		m["proc_uptime"].(prometheus.Gauge).Set(float64(time.Now().Unix() - int64(uptime)))
		io, err := p.IOCounters()
		check(err)
		m["proc_io"].(*prometheus.GaugeVec).WithLabelValues("read_bytes").Set(float64(io.ReadBytes))
		m["proc_io"].(*prometheus.GaugeVec).WithLabelValues("write_bytes").Set(float64(io.WriteBytes))
		mem, err := p.MemoryPercent()
		check(err)
		m["proc_mempct"].(prometheus.Gauge).Set(float64(mem))
		netio, err := p.NetIOCounters(false)
		check(err)
		m["proc_net_io"].(*prometheus.GaugeVec).WithLabelValues("tx_bytes").Set(float64(netio[0].BytesSent))
		m["proc_net_io"].(*prometheus.GaugeVec).WithLabelValues("rx_bytes").Set(float64(netio[0].BytesRecv))
		ctx, err := p.NumCtxSwitches()
		check(err)
		m["proc_ctx"].(*prometheus.GaugeVec).WithLabelValues("voluntary").Set(float64(ctx.Voluntary))
		m["proc_ctx"].(*prometheus.GaugeVec).WithLabelValues("involuntary").Set(float64(ctx.Involuntary))
		fd, err := p.NumFDs()
		check(err)
		m["proc_fd"].(prometheus.Gauge).Set(float64(fd))
		thr, err := p.NumThreads()
		check(err)
		m["proc_threads"].(prometheus.Gauge).Set(float64(thr))
		fls, err := p.OpenFiles()
		check(err)
		m["proc_files"].(prometheus.Gauge).Set(float64(len(fls)))
		time.Sleep(2 * time.Second)
	}

}

func check(e error) {
	return
}

// Returns a JSON schema for the Bunker Grafana Dashboard
func getDashboard() []byte {
	return []byte(`{
		"annotations": {
		  "list": [
			{
			  "builtIn": 1,
			  "datasource": "-- Grafana --",
			  "enable": true,
			  "hide": true,
			  "iconColor": "rgba(0, 211, 255, 1)",
			  "name": "Annotations & Alerts",
			  "type": "dashboard"
			}
		  ]
		},
		"description": "Official Bunker dashboard",
		"editable": true,
		"gnetId": null,
		"graphTooltip": 0,
		"id": 1,
		"links": [],
		"panels": [
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 0,
			  "y": 0
			},
			"hiddenSeries": false,
			"id": 4,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_process_cpu_pct",
				"instant": false,
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Process CPU%",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "percent",
				"label": null,
				"logBase": 1,
				"max": "100",
				"min": null,
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"cacheTimeout": null,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 5,
			  "y": 0
			},
			"hiddenSeries": false,
			"id": 12,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"links": [],
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pluginVersion": "6.5.2",
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": true,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_cluster_network_size",
				"instant": false,
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Cluster Network Size",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": "0",
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": false
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 10,
			  "y": 0
			},
			"hiddenSeries": false,
			"id": 6,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_process_mem_pct",
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Process Memory %",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "percent",
				"label": null,
				"logBase": 1,
				"max": "100",
				"min": null,
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 15,
			  "y": 0
			},
			"hiddenSeries": false,
			"id": 16,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_kv_size_bytes",
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "KV Size",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "decbytes",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 7,
			  "w": 5,
			  "x": 0,
			  "y": 8
			},
			"hiddenSeries": false,
			"id": 8,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "rate(bunker_process_network_io_bytes[5m])",
				"legendFormat": "{{instance}} ({{op}})",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Process Net I/O Bytes",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "Bps",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 7,
			  "w": 5,
			  "x": 5,
			  "y": 8
			},
			"hiddenSeries": false,
			"id": 20,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "sum by (severity) (bunker_log_severity_distribution)",
				"legendFormat": "{{severity}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Log Severity",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 7,
			  "w": 5,
			  "x": 10,
			  "y": 8
			},
			"hiddenSeries": false,
			"id": 10,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_process_open_files_count",
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Process Open File Count",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": "0",
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 7,
			  "w": 5,
			  "x": 15,
			  "y": 8
			},
			"hiddenSeries": false,
			"id": 24,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_runtime_goroutine_count",
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Goroutine Count",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": "100",
				"min": "0",
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 0,
			  "y": 15
			},
			"hiddenSeries": false,
			"id": 2,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pluginVersion": "6.5.2",
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "sum by (code) (rate(promhttp_metric_handler_requests_total[1m]))",
				"format": "time_series",
				"instant": false,
				"legendFormat": "{{code}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "API Status Codes",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 5,
			  "y": 15
			},
			"hiddenSeries": false,
			"id": 14,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_kv_db_freelist_bytes_total",
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "KV Page Bytes Alloc",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "decbytes",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": "0",
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 10,
			  "y": 15
			},
			"hiddenSeries": false,
			"id": 22,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "bunker_runtime_heap_used_bytes / bunker_runtime_heap_total_bytes",
				"legendFormat": "{{instance}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "Heap Used %",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "percentunit",
				"label": null,
				"logBase": 1,
				"max": "1",
				"min": "0",
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": "100",
				"min": "0",
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  },
		  {
			"aliasColors": {},
			"bars": false,
			"dashLength": 10,
			"dashes": false,
			"datasource": null,
			"fill": 1,
			"fillGradient": 0,
			"gridPos": {
			  "h": 8,
			  "w": 5,
			  "x": 15,
			  "y": 15
			},
			"hiddenSeries": false,
			"id": 18,
			"legend": {
			  "avg": false,
			  "current": false,
			  "max": false,
			  "min": false,
			  "show": true,
			  "total": false,
			  "values": false
			},
			"lines": true,
			"linewidth": 1,
			"nullPointMode": "null",
			"options": {
			  "dataLinks": []
			},
			"percentage": false,
			"pointradius": 2,
			"points": false,
			"renderer": "flot",
			"seriesOverrides": [],
			"spaceLength": 10,
			"stack": false,
			"steppedLine": false,
			"targets": [
			  {
				"expr": "avg by (type) (bunker_kv_transaction_time_ms)",
				"legendFormat": "{{type}}",
				"refId": "A"
			  }
			],
			"thresholds": [],
			"timeFrom": null,
			"timeRegions": [],
			"timeShift": null,
			"title": "KV Operation Time",
			"tooltip": {
			  "shared": true,
			  "sort": 0,
			  "value_type": "individual"
			},
			"type": "graph",
			"xaxis": {
			  "buckets": null,
			  "mode": "time",
			  "name": null,
			  "show": true,
			  "values": []
			},
			"yaxes": [
			  {
				"format": "ms",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  },
			  {
				"format": "short",
				"label": null,
				"logBase": 1,
				"max": null,
				"min": null,
				"show": true
			  }
			],
			"yaxis": {
			  "align": false,
			  "alignLevel": null
			}
		  }
		],
		"refresh": "5s",
		"schemaVersion": 21,
		"style": "dark",
		"tags": [],
		"templating": {
		  "list": []
		},
		"time": {
		  "from": "now-15m",
		  "to": "now"
		},
		"timepicker": {
		  "refresh_intervals": [
			"5s",
			"10s",
			"30s",
			"1m",
			"5m",
			"15m",
			"30m",
			"1h",
			"2h",
			"1d"
		  ]
		},
		"timezone": "",
		"title": "Bunker Dashboard",
		"uid": "G-oS_aCWk",
		"version": 1
	  }`)
}
