package msvc

import (
	"context"
	"fmt"
	"github.com/arikkfir/msvc/adapter"
	"github.com/arikkfir/msvc/util"
	kitlog "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	stdlog "log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	EnvDevelopment = iota
	EnvProduction
	contextMsKey = "__ms"
)

func GetFromContext(ctx context.Context) *MicroService {
	value := ctx.Value(contextMsKey)
	if value == nil {
		return nil
	} else {
		return value.(*MicroService)
	}
}

func SetInContext(ctx context.Context, ms *MicroService) context.Context {
	return context.WithValue(ctx, contextMsKey, ms)
}

type Method func(ctx context.Context, request interface{}) (interface{}, error)

type Middleware func(ms *MicroService, methodName string, method Method) Method

type Daemon func() error

type MicroService struct {
	config       interface{}
	environment  int
	log          kitlog.Logger
	methods      map[string]adapter.MethodAdapter
	daemons      []Daemon
	middlewares  []Middleware
	methodChains map[string]Method
	name         string
}

func New(name string, config interface{}) (*MicroService, error) {
	prefix := strings.ToUpper(name)

	// Determine the environment
	environment := EnvDevelopment
	envName := "dev"
	switch env := os.Getenv(prefix + "_ENV"); strings.ToLower(env) {
	case "prod", "production", "prd":
		environment = EnvProduction
		envName = "prod"
	}

	// Log level

	// Configure stdout
	var logger kitlog.Logger
	stdoutWriter := kitlog.NewSyncWriter(os.Stdout)
	if environment == EnvDevelopment {
		logger = kitlog.NewLogfmtLogger(stdoutWriter)
	} else {
		logger = kitlog.NewJSONLogger(stdoutWriter)
	}
	logger = kitlog.With(logger, "svc", name)
	logger = kitlog.With(logger, "env", envName)
	logger = kitlog.With(logger, "ts", kitlog.DefaultTimestamp)
	switch logLevel := os.Getenv(prefix + "_LOGLEVEL"); strings.ToLower(logLevel) {
	case "debug":
		logger = level.NewFilter(logger, level.AllowDebug())
	case "info":
		logger = level.NewFilter(logger, level.AllowInfo())
	case "warn":
		logger = level.NewFilter(logger, level.AllowWarn())
	case "error":
		logger = level.NewFilter(logger, level.AllowError())
	default:
		logger = level.NewFilter(logger, level.AllowAll())
	}
	logger = util.CreateStackTraceLoggerFunc(stdoutWriter, logger)
	stdlog.SetOutput(kitlog.NewStdlibAdapter(logger))

	// Load application configuration
	v := viper.New()
	v.SetConfigName(name)
	v.AddConfigPath(".")
	v.AddConfigPath(fmt.Sprintf("/etc/%s/", name))
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetEnvPrefix(prefix)
	if err := v.Unmarshal(config); err != nil {
		return nil, errors.Wrap(err, "failed reading configuration")
	}

	// Create the service
	return &MicroService{
		config:       config,
		environment:  environment,
		log:          logger,
		name:         name,
		daemons:      make([]Daemon, 0),
		middlewares:  make([]Middleware, 0),
		methods:      make(map[string]adapter.MethodAdapter, 0),
		methodChains: make(map[string]Method, 0),
	}, nil
}

func (ms *MicroService) Config() interface{} {
	return ms.config
}

func (ms *MicroService) Environment() int {
	return ms.environment
}

func (ms *MicroService) Log(kv ...interface{}) {
	_ = ms.log.Log(kv...)
}

func (ms *MicroService) Name() string {
	return ms.name
}

func (ms *MicroService) AddMethod(name string, method interface{}) adapter.MethodAdapter {
	a := adapter.NewAdapter(method)
	ms.methods[name] = a
	ms.compileMethodChains()
	return a
}

func (ms *MicroService) GetMethod(name string) Method {
	return ms.methodChains[name]
}

func (ms *MicroService) GetMethodAdapter(name string) adapter.MethodAdapter {
	return ms.methods[name]
}

func (ms *MicroService) RemoveMethod(name string) {
	delete(ms.methods, name)
	ms.compileMethodChains()
}

func (ms *MicroService) AddMiddleware(middleware Middleware) {
	ms.middlewares = append(ms.middlewares, middleware)
	ms.compileMethodChains()
}

func (ms *MicroService) AddDaemon(daemon Daemon) {
	ms.daemons = append(ms.daemons, daemon)
}

func (ms *MicroService) compileMethodChains() {
	methodChains := map[string]Method{}
	for name, methodAdapter := range ms.methods {
		currentMethod := methodAdapter.Call
		for _, mw := range ms.middlewares {
			currentMethod = mw(ms, name, currentMethod)
		}
		methodChains[name] = currentMethod
	}
	ms.methodChains = methodChains
}

func (ms *MicroService) Run() {

	// Start each daemon in a goroutine; if a daemon fails, it will send the error to the errors channel (termination)
	daemonsWaitGroup := sync.WaitGroup{}
	daemonsWaitGroup.Add(len(ms.daemons))
	errChan := make(chan error, 1)
	doneChan := make(chan bool, 1)
	for _, d := range ms.daemons {
		daemon := d
		go func() {
			if err := daemon(); err != nil {
				errChan <- errors.Wrapf(err, "daemon failed")
			} else {
				daemonsWaitGroup.Done()
			}
		}()
	}

	// Start a goroutine which will wait until all daemons exit successfully, and if so notify the "done" channel
	go func() {
		daemonsWaitGroup.Wait()
		doneChan <- true
	}()

	// Listen for OS signals SIGINT and SIGTERM
	signalsChan := make(chan os.Signal, 1)
	signal.Notify(signalsChan, syscall.SIGINT, syscall.SIGTERM)

	//
	// Wait until we get an error or a signal, print it and exit
	select {
	case err := <-errChan:
		ms.Log("err", err)
		os.Exit(1)
	case sig := <-signalsChan:
		ms.Log("msg", "received signal '"+sig.String()+"'")
		os.Exit(1)
	case <-doneChan:
		ms.Log("msg", "done")
		os.Exit(0)
	}
}
