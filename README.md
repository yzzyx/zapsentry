zapsentry
=========

Logging to sentry via zap


Basic setup
------------

Installation:

	go get github.com/yzzyx/zapsentry
	
	
Usage:

```go
client, err := sentry.NewClient(sentry.ClientOptions{Dsn: "https://xxxx:yyyy@127.0.0.1/1"})
scope := sentry.NewScope()
hub := sentry.NewHub(client, scope)

sentryCore, err := zapsentry.NewCore(hub, zap.InfoLevel)
if err != nil {
	return nil, err
}

logger := zap.New(core)

// Log to sentry with tag 'version' set to 1, and with additional field "somefield" set to "somevalue"
logger.Error("my error", zap.Int("#version", 1), zap.String("somefield", "somevalue"))
```
	
Advanced setup
--------------

Usually a more advanced setup is required. Maybe some default scope values should be set, or logging should
be done both to file and to sentry.

Below a sample setup logging both to sentry, file and console is shown, with a couple of standard fields added,
and some go-sentry integrations disabled.

```go
pe := zap.NewProductionEncoderConfig()
fileEncoder := zapcore.NewJSONEncoder(pe)

logFile, err := os.Create("my-project.log")
if err != nil {
	panic(err)
}

pe.EncodeTime = zapcore.ISO8601TimeEncoder
consoleEncoder := zapcore.NewConsoleEncoder(pe)

client, err := sentry.NewClient(sentry.ClientOptions{
	Dsn:   "http://xxx:yyy@sentry-host/2",
	// Skip default integrations, we set our own tags instead
	Integrations: func(defaultIntegrations []sentry.Integration) []sentry.Integration {
		integrations := defaultIntegrations[:0]
		for k := range defaultIntegrations {
			if defaultIntegrations[k].Name() == "Modules" ||
				defaultIntegrations[k].Name() == "Environment" ||
				defaultIntegrations[k].Name() == "ContextifyFrames" {
				continue
			}
			integrations = append(integrations, defaultIntegrations[k])
		}
		return integrations
	},
})

scope := sentry.NewScope()
scope.SetTag("version", VERSION_NUMBER)
hostname, err := os.Hostname()
if err != nil {
	panic(err)
}

scope.SetTag("server_name", hostname)
scope.SetTag("device.arch", runtime.GOARCH)
scope.SetTag("os.name", runtime.GOOS)

scope.SetTag("runtime.name", "go")
scope.SetTag("runtime.version", runtime.Version())
hub := sentry.NewHub(client, scope)

sentryCore, err := NewCore(hub, level)
if err != nil {
	panic(err)
}

core := zapcore.NewTee(
	zapcore.NewCore(fileEncoder, zapcore.AddSync(f), level),
	zapcore.NewCore(consoleEncoder, IgnoreSync(os.Stdout), level),
	sentryCore,
)

l := zap.New(core)
```	
	
