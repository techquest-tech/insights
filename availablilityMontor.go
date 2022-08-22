package insights

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/spf13/viper"
	"github.com/techquest-tech/gin-shared/pkg/schedule"
	"go.uber.org/zap"
)

type AvailableRequest struct {
	Target string
	Name   string
}

func InitAvailability(logger *zap.Logger) (*AvailabilityMonitorService, error) {
	settings := viper.Sub("monitor.available")
	if settings == nil {
		return nil, fmt.Errorf("missing config for monitor")
	}
	s := &AvailabilityMonitorService{
		Cron:   "@every 30s",
		Logger: logger,
	}
	settings.Unmarshal(s)

	settings = viper.Sub("monitor.azure")
	azuresettings := AppInsightsSettings{}
	if settings != nil {
		settings.Unmarshal(&azuresettings)
		s.AppInsightsSettings = azuresettings
	}

	if keyFromenv := os.Getenv("APPINSIGHTS_INSTRUMENTATIONKEY"); keyFromenv != "" {
		s.Key = keyFromenv
		logger.Info("read application insights key from ENV")
	}
	if s.Key == "" {
		return nil, fmt.Errorf("application insights is empty")
	}

	if len(s.Tests) == 0 {
		s.Tests = make([]AvailableRequest, 1)
		s.Tests[0] = AvailableRequest{Name: "local", Target: "http://127.0.0.1:5000/healthz"}
	}
	return s, nil
}

type AvailabilityMonitorService struct {
	AppInsightsSettings
	Logger *zap.Logger
	Cron   string
	Tests  []AvailableRequest
}

func (ass *AvailabilityMonitorService) GetClient() http.Client {
	return http.Client{
		Timeout: 5 * time.Second,
	}
}

func (ass *AvailabilityMonitorService) triggerTest(req AvailableRequest) {
	start := time.Now()
	client := ass.GetClient()
	_, err := client.Get(req.Target)

	dur := time.Since(start)
	a9y := appinsights.NewAvailabilityTelemetry(req.Name, dur, err == nil)

	msg := ""
	if err != nil {
		ass.Logger.Warn("target return error", zap.String("name", req.Name),
			zap.String("target url", req.Target), zap.Error(err))
		msg = err.Error()
	} else {
		ass.Logger.Info("target return OK", zap.String("name", req.Name),
			zap.String("target url", req.Target), zap.Duration("duration", dur))
	}
	a9y.Message = msg
	aclient := appinsights.NewTelemetryClient(ass.Key)
	aclient.Track(a9y)
}

func (aas *AvailabilityMonitorService) Start() {
	schedule.CreateSchedule("availablilityMonitor", aas.Cron, func() {
		for _, item := range aas.Tests {
			aas.triggerTest(item)
		}
	}, aas.Logger)
}
