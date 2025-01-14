package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"go.uber.org/zap/zapcore"

	"github.com/smartcontractkit/chainlink-relay/pkg/loop"
)

// LoggingConfig controls static logging related configuration that is inherited from the chainlink application to the
// given LOOP executable.
type LoggingConfig interface {
	Level() zapcore.Level
	JSONConsole() bool
	UnixTimestamps() bool
}

type loggingConfig struct {
	level          zapcore.Level
	jsonConsole    bool
	unixTimestamps bool
}

func NewLoggingConfig(level zapcore.Level, jsonConsole bool, unixTimestamps bool) LoggingConfig {
	return &loggingConfig{
		level:          level,
		jsonConsole:    jsonConsole,
		unixTimestamps: unixTimestamps,
	}
}

func (lc *loggingConfig) Level() zapcore.Level {
	return lc.level
}

func (lc *loggingConfig) JSONConsole() bool {
	return lc.jsonConsole
}

func (lc *loggingConfig) UnixTimestamps() bool {
	return lc.unixTimestamps
}

// RegistrarConfig generates contains static configuration inher
type RegistrarConfig interface {
	LoggingConfig
	RegisterLOOP(loopId string, cmdName string) (func() *exec.Cmd, loop.GRPCOpts, error)
}

type registarConfig struct {
	LoggingConfig
	grpcOpts           loop.GRPCOpts
	loopRegistrationFn func(loopId string, loopStaticCfg LoggingConfig) (*RegisteredLoop, error)
}

// NewRegistrarConfig creates a RegistarConfig
// loopRegistrationFn must act as a global registry function of LOOPs and must be idempotent.
// The [func() *exec.Cmd] for a LOOP should be generated by calling [RegistrarConfig.RegisterLOOP]
func NewRegistrarConfig(lc LoggingConfig, grpcOpts loop.GRPCOpts, loopRegistrationFn func(loopId string, loopStaticCfg LoggingConfig) (*RegisteredLoop, error)) RegistrarConfig {
	return &registarConfig{
		LoggingConfig:      lc,
		grpcOpts:           grpcOpts,
		loopRegistrationFn: loopRegistrationFn,
	}
}

// RegisterLOOP calls the configured loopRegistrationFn. The loopRegistrationFn must act as a global registry for LOOPs and must be idempotent.
func (pc *registarConfig) RegisterLOOP(loopID string, cmdName string) (func() *exec.Cmd, loop.GRPCOpts, error) {
	cmdFn, err := NewCmdFactory(pc.loopRegistrationFn, CmdConfig{
		ID:            loopID,
		Cmd:           cmdName,
		LoggingConfig: pc.LoggingConfig,
	})
	if err != nil {
		return nil, loop.GRPCOpts{}, err
	}
	return cmdFn, pc.grpcOpts, nil
}

// EnvConfig is the configuration interface between the application and the LOOP executable. The values
// are fully resolved and static and passed via the environment.
type EnvConfig interface {
	LoggingConfig
	PrometheusPort() int
}

// SetCmdEnvFromConfig sets LOOP-specific vars in the env of the given cmd.
func SetCmdEnvFromConfig(cmd *exec.Cmd, cfg EnvConfig) {
	forward := func(name string) {
		if v, ok := os.LookupEnv(name); ok {
			cmd.Env = append(cmd.Env, name+"="+v)
		}
	}
	forward("CL_DEV")
	forward("CL_LOG_SQL_MIGRATIONS")
	forward("CL_LOG_COLOR")
	cmd.Env = append(cmd.Env,
		"CL_LOG_LEVEL="+cfg.Level().String(),
		"CL_JSON_CONSOLE="+strconv.FormatBool(cfg.JSONConsole()),
		"CL_UNIX_TS="+strconv.FormatBool(cfg.UnixTimestamps()),
		"CL_PROMETHEUS_PORT="+strconv.FormatInt(int64(cfg.PrometheusPort()), 10),
	)
}

// GetEnvConfig deserializes LOOP-specific environment variables to an EnvConfig
func GetEnvConfig() (EnvConfig, error) {
	logLevelStr := os.Getenv("CL_LOG_LEVEL")
	logLevel, err := zapcore.ParseLevel(logLevelStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CL_LOG_LEVEL = %q: %w", logLevelStr, err)
	}
	promPortStr := os.Getenv("CL_PROMETHEUS_PORT")
	promPort, err := strconv.Atoi(promPortStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CL_PROMETHEUS_PORT = %q: %w", promPortStr, err)
	}
	return &envConfig{
		logLevel:       logLevel,
		jsonConsole:    strings.EqualFold("true", os.Getenv("CL_JSON_CONSOLE")),
		unixTimestamps: strings.EqualFold("true", os.Getenv("CL_UNIX_TS")),
		prometheusPort: promPort,
	}, nil
}

// envConfig is an implementation of EnvConfig.
type envConfig struct {
	logLevel       zapcore.Level
	jsonConsole    bool
	unixTimestamps bool
	prometheusPort int
}

func NewEnvConfig(lc LoggingConfig, prometheusPort int) EnvConfig {
	//prometheusPort := prometheusPortFn(name)
	return &envConfig{
		logLevel:       lc.Level(),
		jsonConsole:    lc.JSONConsole(),
		unixTimestamps: lc.UnixTimestamps(),
		prometheusPort: prometheusPort,
	}
}

func (e *envConfig) Level() zapcore.Level {
	return e.logLevel
}

func (e *envConfig) JSONConsole() bool {
	return e.jsonConsole
}

func (e *envConfig) UnixTimestamps() bool {
	return e.unixTimestamps
}

func (e *envConfig) PrometheusPort() int {
	return e.prometheusPort
}
