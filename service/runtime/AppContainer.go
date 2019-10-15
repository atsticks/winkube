package runtime

import (
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/netutil"
	"github.com/winkube/service/util"
	"os"
	"sync"
	"time"
)

var container *AppContainer
var once sync.Once

const WINKUBE_ADTYPE = "winkube-service"

func Container() *AppContainer {
	once.Do(func() {
		container = start()
	})
	return container
}

func start() *AppContainer {
	c := AppContainer{
		Logger: Logger(),
		Config: Config(),
		Router: Router(),
	}
	c.ServiceProvider = ServiceProvider(c.Config)
	c.ServiceRegistry = ServiceRegistry(&c.ServiceProvider, WINKUBE_ADTYPE)
	c.ClusterManager = CreateClusterManager(&c.ServiceRegistry)
	return &c
}

type AppContainer struct {
	Startup         time.Time
	StartupDuration time.Duration
	Logger          log.Logger
	Config          *AppConfiguration
	ServiceProvider netutil.ServiceProvider
	Router          *mux.Router
	ServiceRegistry netutil.ServiceRegistry
	ClusterManager  ClusterManager
}

func (this AppContainer) Stats() string {
	return "Container running (TODO startup and duration)"
}

type DefaultServiceProvider struct {
	config *AppConfiguration
}

func (this DefaultServiceProvider) GetServices() []netutil.Service {
	// TODO implement on base of config and effective state of setup on this machine
	return []netutil.Service{}
}

// Dependeny Injection Module, provides logger and more...
func ServiceProvider(config *AppConfiguration) netutil.ServiceProvider {
	log.Info("Initializing service provider...")
	return DefaultServiceProvider{
		config: config,
	}
}
func Config() *AppConfiguration {
	return CreateAppConfig("winkube.config")
}
func Logger() log.Logger {
	log.Info("Initializing logging...")
	//log.SetFormatter(&log.JSONFormatter{}) // Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(util.NewPlainFormatter())

	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	//log.SetReportCaller(true)
	log.WithFields(log.Fields{
		"app":    "kube-win",
		"node":   netutil.GetDefaultIP(),
		"server": util.RuntimeInfo(),
	})
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	log.Infoln("Working directory: " + dir)
	return *log.StandardLogger()
}
func Router() *mux.Router {
	log.Info("Initializing web application...")
	r := mux.NewRouter()
	return r
}

func ServiceRegistry(serviceProvider *netutil.ServiceProvider, adType string) netutil.ServiceRegistry {
	return netutil.InitServiceRegistry(adType, serviceProvider)
}
