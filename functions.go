package main

import (
	"net"
	"os"

	"github.com/denisbrodbeck/machineid"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// getConfig loads config from its various sources
func getConfig() (*Config, error) {
	id, err := machineid.ID()
	if err != nil {
		return &Config{}, err
	}
	c := &Config{
		Mode: "dev",
		Cluster: ClusterConfig{
			BindPort:      2000,
			DiscoveryHost: "127.0.0.1:2000",
			AdvertiseHost: "127.0.0.1:2000",
		},
		KV: KVConfig{
			Encryption: true,
			DBPath:     "kv.db",
		},
		API: APIConfig{
			Enable:         true,
			Port:           2001,
			Authentication: true,
			EnableMetrics:  true,
		},
		UI: UIConfig{
			Enable:         true,
			Port:           443,
			Authentication: true,
		},
		SSL: SSLConfig{
			Enable:         true,
			SSLCertificate: "my.crt",
			SSLKey:         "my.key",
		},
		Perf: PerfConfig{
			BufferSize: 4096,
		},
	}
	v := viper.New()
	v.SetConfigName("config.yaml")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/yeticloud/bunker")
	v.SetEnvPrefix("BUNKER")
	v.AutomaticEnv()
	flags, err := bindFlags(id)
	if err != nil {
		return c, err
	}
	err = v.BindPFlags(flags)
	if err != nil {
		return c, err
	}
	err = v.ReadInConfig()
	if err != nil {
		return c, err
	}
	err = v.Unmarshal(&c)
	if err != nil {
		return c, nil
	}
	if c.Mode != "prod" && c.Mode != "dev" {
		os.Stderr.WriteString("'mode' must be set to either 'dev' or 'prod'; value '" + c.Mode + "' is not a valid mode.\n")
		os.Exit(2)
	}
	return c, nil
}

func bindFlags(nodeid string) (*pflag.FlagSet, error) {
	fs := pflag.NewFlagSet("Bunker", pflag.ExitOnError)
	fs.SortFlags = true
	fs.BoolP("help", "h", false, "Prints out this usage/help info")
	fs.StringP("mode", "m", "dev", "Sets the operation mode: dev, prod")
	fs.Uint16("cluster.bindport", 2000, "Port to bind to for listening for inbound cluster messages")
	fs.String("cluster.discoveryhost", "127.0.0.1:2000", "Host/IP to announce its presnece to")
	fs.String("cluster.advertisehost", "127.0.0.1:2000", "Host/IP to advertise when connecting to the cluster")
	fs.Bool("kv.encryption", true, "Enable encrypted values in the key-value store")
	fs.String("kv.dbpath", "kv.db", "Path to save the key-value store if disk persistance is enable")
	fs.Bool("api.enable", true, "Enable the REST API")
	fs.Uint16("api.port", 2001, "Port for the REST API to listen on")
	fs.Bool("api.authentication", true, "Enable authentication on the REST API")
	fs.Bool("api.enablemetrics", true, "Enable Prometheus metrics endpoint")
	fs.Bool("ui.enable", true, "Enable the embedded web UI")
	fs.Uint16("ui.port", 443, "Port for the embedded web UI to listen on")
	fs.Bool("ui.authentication", true, "Enable authentication for the embedded web UI")
	fs.Bool("ssl.enable", true, "Enable SSL for the REST API and embedded web UI")
	fs.String("ssl.certificate", "my.crt", "Path to the SSL certificate to use")
	fs.String("ssl.key", "my.key", "Path to the SSL private key to use")
	fs.Uint64("performance.buffersize", 4096, "Internal buffer size")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		return fs, err
	}
	h, err := fs.GetBool("help")
	if err != nil {
		return fs, err
	}
	if h {
		os.Stderr.WriteString("\nBunker is an encrypted, distributed key-value store.\n\n")
		os.Stderr.WriteString("Command-line usage:\n")
		fs.PrintDefaults()
		os.Stderr.WriteString("\nAdditional help and documentation can be found at https://github.com/yeticloud/bunker\n\n")
		os.Exit(2)
	}
	return fs, nil
}

func getIP(addr string) string {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
