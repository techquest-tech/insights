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
	s := &AvailabilityMonitorService{
		Cron:   "@every 5m",
		Logger: logger,
	}
	settings := viper.Sub("tracing.available")
	if settings != nil {
		settings.Unmarshal(s)
	}
	settings = viper.Sub("tracing.azure")
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
	s.client = appinsights.NewTelemetryClient(s.Key)
	return s, nil
}

type AvailabilityMonitorService struct {
	AppInsightsSettings
	Logger *zap.Logger
	Cron   string
	Tests  []AvailableRequest
	client appinsights.TelemetryClient
}

func (ass *AvailabilityMonitorService) GetClient() http.Client {
	return http.Client{
		Timeout: 5 * time.Second,
	}
}

func (ass *AvailabilityMonitorService) triggerTest(req AvailableRequest) {
	start := time.Now()
	client := ass.GetClient()

	resp, err := client.Get(req.Target)

	dur := time.Since(start)
	a9y := appinsights.NewAvailabilityTelemetry(req.Name, dur, err == nil)

	if err != nil {
		ass.Logger.Error("available test failed.", zap.Error(err))
	}
	defer resp.Body.Close()

	msg := ""
	if err != nil {
		ass.Logger.Warn("target return error", zap.String("name", req.Name),
			zap.String("target url", req.Target), zap.Error(err))
		msg = err.Error()
	} else {
		ass.Logger.Debug("target return OK", zap.String("name", req.Name),
			zap.String("target url", req.Target), zap.Duration("duration", dur))
	}
	a9y.Message = msg
	// aclient := appinsights.NewTelemetryClient(ass.Key)
	ass.client.Track(a9y)
}

func (aas *AvailabilityMonitorService) Start() {
	schedule.CreateSchedule("availablilityMonitor", aas.Cron, func() {
		for _, item := range aas.Tests {
			aas.triggerTest(item)
		}
	}, aas.Logger)
}
