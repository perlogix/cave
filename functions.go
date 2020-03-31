package main

import "github.com/spf13/viper"

// GetConfig loads config from its various sources
func GetConfig() (*Config, error) {
	c := &Config{
		Mode: "dev",
		Cluster: Cluster{
			Port:          2000,
			DiscoveryHost: "127.0.0.1",
			AdvertiseHost: "127.0.0.1",
		},
		KV: KV{
			Encryption:  true,
			Persist:     true,
			PersistPath: "kv.db",
		},
		API: API{
			Enable:         true,
			Port:           2001,
			Authentication: true,
			EnableMetrics:  true,
		},
		UI: UI{
			Enable:         true,
			Port:           443,
			Authentication: true,
		},
		SSL: SSL{
			Enable:         true,
			SSLCertificate: "my.crt",
			SSLKey:         "my.key",
		},
	}
	v := viper.New()
	v.SetConfigName("config.yaml")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/yeticloud/bunker")
	v.SetEnvPrefix("BUNKER")
	v.AutomaticEnv()
	err := v.ReadInConfig()
	if err != nil {
		return c, err
	}
	err = v.Unmarshal(&c)
	if err != nil {
		return c, nil
	}
	return c, nil
}
