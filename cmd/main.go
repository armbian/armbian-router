package main

import (
	"flag"
	"github.com/armbian/redirector"
	"github.com/armbian/redirector/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
)

var (
	configFlag = flag.String("config", "", "configuration file path")
	flagDebug  = flag.Bool("debug", false, "Enable debug logging")
)

func main() {
	flag.Parse()

	if *flagDebug {
		log.SetLevel(log.DebugLevel)
	}

	viper.SetDefault("bind", ":8080")
	viper.SetDefault("cacheSize", 1024)
	viper.SetDefault("topChoices", 1)
	viper.SetDefault("maxDeviation", 50*1000) // 50 kilometers
	viper.SetDefault("reloadKey", util.RandomSequence(32))

	viper.SetConfigName("dlrouter")        // name of config file (without extension)
	viper.SetConfigType("yaml")            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/dlrouter/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.dlrouter") // call multiple times to add many search paths
	viper.AddConfigPath(".")               // optionally look for config in the working directory

	if *configFlag != "" {
		viper.SetConfigFile(*configFlag)
	}

	config := &redirector.Config{}

	loadConfig := func(fatal bool) {
		log.Info("Reading configuration")

		// Bind reload to reading in the viper config, then deserializing
		if err := viper.ReadInConfig(); err != nil {
			log.WithError(err).Error("Unable to unmarshal configuration")

			if fatal {
				os.Exit(1)
			}
		}

		log.Info("Unmarshalling configuration")

		if err := viper.Unmarshal(config); err != nil {
			log.WithError(err).Error("Unable to unmarshal configuration")

			if fatal {
				os.Exit(1)
			}
		}

		log.Info("Updating root certificates")

		certs, err := util.LoadCACerts()

		if err != nil {
			log.WithError(err).Error("Unable to load certificates")

			if fatal {
				os.Exit(1)
			}
		}

		config.SetRootCAs(certs)
	}

	config.ReloadFunc = func() {
		loadConfig(false)
	}

	loadConfig(true)

	redir := redirector.New(config)

	// Because we have a bind address, we can start it without the return value.
	redir.Start()

	log.Info("Ready")

	c := make(chan os.Signal)

	signal.Notify(c, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGHUP)

	for {
		sig := <-c

		if sig != syscall.SIGHUP {
			break
		}

		loadConfig(false)

		err := redir.ReloadConfig()

		if err != nil {
			log.WithError(err).Warning("Did not reload configuration due to error")
		}
	}
}
