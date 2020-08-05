package main

import (
	"fmt"
	"github.com/MagalixCorp/magalix-agent/v2/client"
	"github.com/MagalixCorp/magalix-agent/v2/entities"
	"github.com/MagalixCorp/magalix-agent/v2/executor"
	"github.com/MagalixCorp/magalix-agent/v2/kuber"
	"github.com/MagalixCorp/magalix-agent/v2/metrics"
	"github.com/MagalixCorp/magalix-agent/v2/proto"
	"github.com/MagalixCorp/magalix-agent/v2/scanner"
	"github.com/MagalixCorp/magalix-agent/v2/utils"
	"github.com/MagalixTechnologies/log-go"
	"github.com/MagalixTechnologies/uuid-go"
	"github.com/docopt/docopt-go"
	"github.com/reconquest/karma-go"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"time"
)

var usage = `agent - magalix services agent.

Usage:
  agent -h | --help
  agent [options] (--kube-url= | --kube-incluster) [--skip-namespace=]... [--source=]...

Options:
  --gateway <address>                        Connect to specified Magalix Kubernetes Agent gateway.
                                              [default: wss://gateway.agent.magalix.cloud]
  --account-id <identifier>                  Your account ID in Magalix.
                                              [default: $ACCOUNT_ID]
  --cluster-id <identifier>                  Your cluster ID in Magalix.
                                              [default: $CLUSTER_ID]
  --client-secret <secret>                   Unique and secret client token.
                                              [default: $SECRET]
  --kube-url <url>                           Use specified URL and token for access to kubernetes
                                              cluster.
  --kube-insecure                            Insecure skip SSL verify.
  --kube-root-ca-cert <filepath>             Filepath to root CA cert.
  --kube-token <token>                        Use specified token for access to kubernetes cluster.
  --kube-incluster                           Automatically determine kubernetes client set
                                              configuration. Works only if program is
                                              running inside kubernetes cluster.
  --kube-timeout <duration>                  Timeout of requests to kubernetes apis.
                                              [default: 20s]
  --skip-namespace <pattern>                 Skip namespace matching a pattern (e.g. system-*),
                                              can be specified multiple times.
  --source <source>                          Specify source for metrics instead of
                                              automatically detected.
                                              Supported sources are:
                                              * kubelet;
  --kubelet-port <port>                      Override kubelet port for
                                              automatically discovered nodes.
                                              [default: 10255]
  --kubelet-backoff-sleep <duration>         Timeout of backoff policy.
                                              Timeout will be multiplied from 1 to 10.
                                              [default: 300ms]
  --kubelet-backoff-max-retries <retries>    Max retries of backoff policy, then consider failed.
                                              [default: 5]
  --metrics-interval <duration>              Metrics request and send interval.
                                              [default: 1m]
  --events-buffer-flush-interval <duration>  Events batch writer flush interval.
                                              [default: 10s]
  --events-buffer-size <size>                Events batch writer buffer size.
                                              [default: 20]
  --executor-workers <number>                 Executor concurrent workers count
                                              [default: 5]
  --timeout-proto-handshake <duration>       Timeout to do a websocket handshake.
                                              [default: 10s]
  --timeout-proto-write <duration>           Timeout to write a message to websocket channel.
                                              [default: 60s]
  --timeout-proto-read <duration>            Timeout to read a message from websocket channel.
                                              [default: 60s]
  --timeout-proto-reconnect <duration>       Timeout between reconnecting retries.
                                              [default: 1s]
  --timeout-proto-backoff <duration>         Timeout of backoff policy.
                                              Timeout will be multiplied from 1 to 10.
                                              [default: 300ms]
  --opt-in-analysis-data                     Send anonymous data for analysis.
  --analysis-data-interval <duration>        Analysis data send interval.
                                              [default: 5m]
  --packets-v2                               Enable v2 packets (without ids). This is deprecated and kept for backward compatibility.
  --disable-metrics                          Disable metrics collecting and sending.
  --disable-events                           Disable events collecting and sending.
  --disable-scalar                           Disable in-agent scalar.
  --port <port>                              Port to start the server on for liveness and readiness probes
                                               [default: 80]
  --dry-run                                  Disable decision execution.
  --no-send-logs                             Disable sending logs to the backend.
  --debug                                    Enable debug messages.
  --trace                                    Enable debug and trace messages.
  --trace-log <path>                         Write log messages to specified file
                                              [default: trace.log]
  -h --help                                  Show this help.
  --version                                  Show version.
`

var version = "[manual build]"

type Args struct {
	Gateway      string `docopt:"--gateway"`
	AccountId    string `docopt:"--account-id"`
	ClusterId    string `docopt:"--cluster-id"`
	ClientSecret string `docopt:"--client-secret"`

	KubeUrl        string `docopt:"--kube-url"`
	KubeInsecure   bool   `docopt:"--kube-insecure"`
	KubeRootCaCert string `docopt:"--kube-root-ca-cert"`
	KubeToken      string `docopt:"--kube-token"`
	KubeInCluster  bool   `docopt:"--kube-incluster"`
	KubeTimeout    string `docopt:"--kube-timeout"`

	SkipNamespaces []string `docopt:"--skip-namespace"`
	MetricsSources []string `docopt:"--source"`

	KubeletPort              int    `docopt:"--kubelet-port"`
	KubeletBackoffMaxRetries int    `docopt:"--kubelet-backoff-max-retries"`
	KubeletBackoffSleep      string `docopt:"--kubelet-backoff-sleep"`

	MetricsInterval           string `docopt:"--metrics-interval"`
	EventsBufferFlushInterval string `docopt:"--events-buffer-flush-interval"`
	EventsBufferSize          int    `docopt:"--events-buffer-size"`
	ExecutorWorkers           int    `docopt:"--executor-workers"`

	TimeoutProtoHandshake string `docopt:"--timeout-proto-handshake"`
	TimeoutProtoWrite     string `docopt:"--timeout-proto-write"`
	TimeoutProtoRead      string `docopt:"--timeout-proto-read"`
	TimeoutProtoReconnect string `docopt:"--timeout-proto-reconnect"`
	TimeoutProtoBackoff   string `docopt:"--timeout-proto-backoff"`

	OptInAnalysisData    bool   `docopt:"--opt-in-analysis-data"`
	AnalysisDataInterval string `docopt:"--analysis-data-interval"`

	DisableMetrics bool `docopt:"--disable-metrics"`
	DisableEvents  bool `docopt:"--disable-events"`
	DisableScalar  bool `docopt:"--disable-scalar"`
	NoSendLogs     bool `docopt:"--no-send-logs"`

	PacketsV2 bool `docopt:"--packets-v2"`

	Port int `docopt:"--port"`

	DryRun bool `docopt:"--dry-run"`

	Debug    bool   `docopt:"--debug"`
	Trace    bool   `docopt:"--trace"`
	TraceLog string `docopt:"--trace-log"`
}

const (
	entitiesSyncTimeout = time.Minute
)

func getVersion() string {
	return strings.Join([]string{
		"magalix agent " + version,
		"protocol/major: " + fmt.Sprint(client.ProtocolMajorVersion),
		"protocol/minor: " + fmt.Sprint(client.ProtocolMinorVersion),
	}, "\n")
}

func getConfig(args *Args, startId uuid.UUID) *utils.AgentConfig {
	utils.ExpandEnv(&args.AccountId)
	utils.ExpandEnv(&args.ClusterId)
	utils.ExpandEnv(&args.ClientSecret)

	return &utils.AgentConfig{
		Gateway:                   args.Gateway,
		StartId:                   startId,
		AccountId:                 utils.ParseUuidString(args.AccountId),
		ClusterId:                 utils.ParseUuidString(args.ClusterId),
		ClientSecret:              utils.MustDecodeSecret(args.ClientSecret),
		KubeUrl:                   args.KubeUrl,
		KubeInsecure:              args.KubeInsecure,
		KubeRootCaCert:            args.KubeRootCaCert,
		KubeToken:                 args.KubeToken,
		KubeInCluster:             args.KubeInCluster,
		KubeTimeout:               utils.MustParseDuration(args.KubeTimeout),
		SkipNamespaces:            args.SkipNamespaces,
		MetricsSources:            args.MetricsSources,
		MetricsInterval:           utils.MustParseDuration(args.MetricsInterval),
		EventsBufferFlushInterval: utils.MustParseDuration(args.EventsBufferFlushInterval),
		EventsBufferSize:          args.EventsBufferSize,
		ExecutorWorkers:           args.ExecutorWorkers,
		KubeletPort:               args.KubeletPort,
		KubeletBackoffMaxRetries:  args.KubeletBackoffMaxRetries,
		KubeletBackoffSleep:       utils.MustParseDuration(args.KubeletBackoffSleep),
		TimeoutProtoHandshake:     utils.MustParseDuration(args.TimeoutProtoHandshake),
		TimeoutProtoWrite:         utils.MustParseDuration(args.TimeoutProtoBackoff),
		TimeoutProtoRead:          utils.MustParseDuration(args.TimeoutProtoRead),
		TimeoutProtoReconnect:     utils.MustParseDuration(args.TimeoutProtoReconnect),
		TimeoutProtoBackoff:       utils.MustParseDuration(args.TimeoutProtoBackoff),
		OptInAnalysisData:         args.OptInAnalysisData,
		AnalysisDataInterval:      utils.MustParseDuration(args.AnalysisDataInterval),
		MetricsEnabled:            !args.DisableMetrics,
		EventsEnabled:             !args.DisableEvents,
		SendLogs:                  !args.NoSendLogs,
		Port:                      args.Port,
		DryRun:                    args.DryRun,
		Debug:                     args.Debug,
		Trace:                     args.Trace,
		TraceLog:                  args.TraceLog,
		Version:                   version,
	}
}

func main() {
	opts, err := docopt.ParseArgs(usage, nil, getVersion())
	if err != nil {
		panic(err)
	}

	args := &Args{}
	err = opts.Bind(args)
	if err != nil {
		panic(err)
	}

	logger := log.New(
		args.Debug,
		args.Trace,
		args.TraceLog,
	)

	// we need to disable default exit 1 for FATAL messages because we also
	// need to send fatal messages on the remote server and send bye packet
	// after fatal message (if we can), therefore all exits will be controlled
	// manually
	logger.SetExiter(func(int) {})
	utils.SetLogger(logger)

	logger.Infof(
		karma.Describe("version", version).
			Describe("args", fmt.Sprintf("%q", utils.GetSanitizedArgs())),
		"magalix agent started.....",
	)

	startID := uuid.NewV4()
	conf := getConfig(args, startID)

	// TODO: remove
	// a hack to set default timeout for all http requests
	http.DefaultClient = &http.Client{
		Timeout: 20 * time.Second,
	}

	probes := NewProbesServer(":"+strconv.Itoa(args.Port), logger)
	go func() {
		err = probes.Start()
		if err != nil {
			logger.Fatalf(err, "unable to start probes server")
			os.Exit(1)
		}
	}()

	connected := make(chan bool)
	gwClient, err := client.InitClient(
		conf,
		logger,
		connected)

	defer gwClient.WaitExit()
	defer gwClient.Recover()

	if err != nil {
		logger.Fatalf(err, "unable to connect to gateway")
		os.Exit(1)
	}

	logger.Infof(nil, "Waiting for connection and authorization")
	<-connected
	logger.Infof(nil, "Connected and authorized")
	probes.Authorized = true
	initAgent(conf, gwClient, logger)
}

func initAgent(conf *utils.AgentConfig, gwClient *client.Client, logger *log.Logger) {
	logger.Infof(nil, "Initializing Agent")

	kRestConfig, err := getKRestConfig(logger, conf)

	dynamicClient, err := dynamic.NewForConfig(kRestConfig)
	parentsStore := kuber.NewParentsStore()
	observer := kuber.NewObserver(
		logger,
		dynamicClient,
		parentsStore,
		make(chan struct{}, 0),
		time.Minute*5,
	)
	t := entitiesSyncTimeout
	err = observer.WaitForCacheSync(&t)
	if err != nil {
		logger.Fatalf(err, "unable to start entities watcher")
	}

	kube, err := kuber.InitKubernetes(kRestConfig, gwClient)
	if err != nil {
		logger.Fatalf(err, "unable to initialize Kubernetes")
		os.Exit(1)
	}

	var entityScanner *scanner.Scanner

	ew := entities.NewEntitiesWatcher(logger, observer, gwClient)
	err = ew.Start()
	if err != nil {
		logger.Fatalf(err, "unable to start entities watcher")
	}

	entityScanner = scanner.InitScanner(
		conf,
		gwClient,
		scanner.NewKuberFromObserver(ew),
	)

	e := executor.InitExecutor(
		gwClient,
		kube,
		entityScanner,
		conf.ExecutorWorkers,
		conf.DryRun,
	)

	gwClient.AddListener(proto.PacketKindDecision, e.Listener)
	gwClient.AddListener(proto.PacketKindRestart, func(in []byte) (out []byte, err error) {
		var restart proto.PacketRestart
		if err = proto.DecodeSnappy(in, &restart); err != nil {
			return
		}
		defer gwClient.Done(restart.Status, true)
		return nil, nil
	})

	// @TODO re-allow events when we start using them
	//if conf.EventsEnabled {
	//
	//	events.InitEvents(
	//		conf,
	//		gwClient,
	//		kube,
	//		entityScanner,
	//	)
	//}

	if conf.MetricsEnabled {

		var nodesProvider metrics.NodesProvider
		var entitiesProvider metrics.EntitiesProvider

		nodesProvider = observer
		entitiesProvider = observer

		err := metrics.InitMetrics(
			conf,
			gwClient,
			nodesProvider,
			entitiesProvider,
			kube,
		)
		if err != nil {
			gwClient.Fatalf(err, "unable to initialize metrics sources")
			os.Exit(1)
		}
	}

	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
}

func getKRestConfig(
	logger *log.Logger,
	agentConfig *utils.AgentConfig,
) (config *rest.Config, err error) {
	if agentConfig.KubeInCluster {
		logger.Infof(nil, "initializing kubernetes incluster config")

		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, karma.Format(
				err,
				"unable to get incluster config",
			)
		}

	} else {
		logger.Infof(
			nil,
			"initializing kubernetes user-defined config",
		)

		if agentConfig.KubeToken == "" {
			agentConfig.KubeToken = os.Getenv("KUBE_TOKEN")
		}

		config = &rest.Config{}
		config.ContentType = runtime.ContentTypeJSON
		config.APIPath = "/api"
		config.Host = agentConfig.KubeUrl
		config.BearerToken = agentConfig.KubeToken

		{
			tlsClientConfig := rest.TLSClientConfig{}
			if _, err := cert.NewPool(agentConfig.KubeRootCaCert); err != nil {
				fmt.Printf("Expected to load root CA config from %s, but got err: %v", agentConfig.KubeRootCaCert, err)
			} else {
				tlsClientConfig.CAFile = agentConfig.KubeRootCaCert
			}
			config.TLSClientConfig = tlsClientConfig
		}

		if agentConfig.KubeInsecure {
			config.Insecure = true
		}
	}

	config.Timeout = agentConfig.KubeTimeout

	return
}
