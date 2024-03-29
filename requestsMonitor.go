package insights

import (
	"fmt"
	"os"

	"github.com/asaskevich/EventBus"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/spf13/viper"
	"github.com/techquest-tech/cronext"
	"github.com/techquest-tech/gin-shared/pkg/core"
	"github.com/techquest-tech/gin-shared/pkg/event"
	"github.com/techquest-tech/gin-shared/pkg/ginshared"
	"github.com/techquest-tech/gin-shared/pkg/tracing"
	"go.uber.org/zap"
)

type AppInsightsSettings struct {
	Key     string
	Role    string
	Version string
	Details bool
}

type ResquestMonitor struct {
	AppInsightsSettings
	logger *zap.Logger
	// client appinsights.TelemetryClient
}

func InitRequestMonitor(logger *zap.Logger, bus EventBus.Bus) *ResquestMonitor {
	azureSetting := AppInsightsSettings{
		Role:    core.AppName,
		Version: core.Version,
	}
	client := &ResquestMonitor{
		logger: logger,
	}
	settings := viper.Sub("tracing.azure")
	if settings != nil {
		settings.Unmarshal(&azureSetting)
	}
	client.AppInsightsSettings = azureSetting
	if keyFromenv := os.Getenv("APPINSIGHTS_INSTRUMENTATIONKEY"); keyFromenv != "" {
		client.Key = keyFromenv
		logger.Info("read application insights key from ENV")
	}

	if client.Key == "" {
		logger.Warn("no application insights key provided, tracing function disabled.")
		return nil
	}

	bus.SubscribeAsync(event.EventError, client.ReportError, false)
	bus.SubscribeAsync(event.EventTracing, client.ReportTracing, false)
	bus.SubscribeAsync(cronext.EventJobFinished, client.ReportScheduleJob, false)
	logger.Info("event subscribed for application insights", zap.Bool("details", client.Details))
	client.getClient()
	return client
}

func (appins *ResquestMonitor) ReportScheduleJob(req cronext.JobHistory) {
	status := 200
	if !req.Succeed {
		status = 500
	}

	details := &tracing.TracingDetails{
		Uri:     req.Job,
		Method:  "Cron",
		Durtion: req.Duration,
		Status:  status,
	}
	appins.ReportTracing(details)
}

func (appins *ResquestMonitor) getClient() appinsights.TelemetryClient {
	// if appins.client == nil {
	client := appinsights.NewTelemetryClient(appins.Key)
	if appins.Role != "" {
		client.Context().Tags.Cloud().SetRole(appins.Role)
	}
	if appins.Version != "" {
		client.Context().Tags.Application().SetVer(appins.Version)
	}
	// appins.client = client
	return client
	// }
	// return appins.client
}

func (appins *ResquestMonitor) ReportError(err error) {
	client := appins.getClient()
	trace := appinsights.NewTraceTelemetry(err.Error(), appinsights.Error)
	client.Track(trace)
	appins.logger.Debug("tracing error done", zap.Error(err))
}

func (appins *ResquestMonitor) ReportTracing(tr *tracing.TracingDetails) {
	client := appins.getClient()

	client.Context().Tags.Operation().SetName(fmt.Sprintf("%s %s", tr.Method, tr.Optionname))

	t := appinsights.NewRequestTelemetry(
		tr.Method, tr.Uri, tr.Durtion, fmt.Sprintf("%d", tr.Status),
	)

	t.Source = tr.ClientIP
	t.Properties["user-agent"] = tr.UserAgent
	t.Properties["device"] = tr.Device
	if tr.Body != "" {
		if appins.Details {
			t.Properties["req"] = tr.Body
		}
		t.Measurements["body-size"] = float64(len(tr.Body))
	}
	if tr.Resp != "" {
		if appins.Details {
			t.Properties["resp"] = tr.Resp
		}
		t.Measurements["resp-size"] = float64(len(tr.Resp))
	}

	client.Track(t)
	appins.logger.Debug("submit tracing done.")
}

func EnabledMonitor() {
	// tracing.EnabledTracing()
	core.ProvideStartup(func(t *tracing.TracingRequestService, logger *zap.Logger, bus EventBus.Bus) core.Startup {
		InitRequestMonitor(logger, bus)
		return nil
	})
}

func EnabledAvailability() {
	ginshared.Provide(InitAvailability)
	ginshared.GetContainer().Invoke(func(service *AvailabilityMonitorService) error {
		service.Start()
		return nil
	})
}

func Enabled() {
	EnabledMonitor()
	EnabledAvailability()
}
