package utils

import (
	"github.com/MagalixTechnologies/uuid-go"
	"time"
)

type AgentConfig struct {
	Gateway string

	StartId      uuid.UUID
	AccountId    uuid.UUID
	ClusterId    uuid.UUID
	ClientSecret []byte

	KubeUrl        string
	KubeInsecure   bool
	KubeRootCaCert string
	KubeToken      string
	KubeInCluster  bool
	KubeTimeout    time.Duration

	SkipNamespaces            []string
	MetricsSources            []string
	MetricsInterval           time.Duration
	EventsBufferFlushInterval time.Duration
	EventsBufferSize          int
	ExecutorWorkers           int

	KubeletPort              int
	KubeletBackoffMaxRetries int
	KubeletBackoffSleep      time.Duration

	TimeoutProtoHandshake time.Duration
	TimeoutProtoWrite     time.Duration
	TimeoutProtoRead      time.Duration
	TimeoutProtoReconnect time.Duration
	TimeoutProtoBackoff   time.Duration

	OptInAnalysisData    bool
	AnalysisDataInterval time.Duration

	MetricsEnabled bool
	EventsEnabled  bool
	ScalarEnabled  bool
	SendLogs       bool

	Port int

	DryRun bool

	Debug    bool
	Trace    bool
	TraceLog string

	Version string
}

