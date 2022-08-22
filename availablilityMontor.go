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

func InitAvailablitity(logger *zap.Logger) (*AvailablilityMonitorService, error) {
	settings := viper.Sub("monitor.available")
	if settings == nil {
		return nil, fmt.Errorf("missing config for monitor")
	}
	s := &AvailablilityMonitorService{}
	settings.Unmarshal(s)
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

type AvailablilityMonitorService struct {
	AppInsightsSettings `json:"azure"`
	Logger              *zap.Logger
	Cron                string
	Tests               []AvailableRequest
}

func (ass *AvailablilityMonitorService) GetClient() http.Client {
	return http.Client{
		Timeout: 5 * time.Second,
	}
}

func (ass *AvailablilityMonitorService) triggerTest(req AvailableRequest) {
	start := time.Now()
	client := ass.GetClient()
	_, err := client.Get(req.Target)

	dur := time.Since(start)
	a9y := appinsights.NewAvailabilityTelemetry(req.Name, dur, err == nil)

	msg := ""
	if err != nil {
		msg = err.Error()
	}
	a9y.Message = msg
	aclient := appinsights.NewTelemetryClient(ass.Key)
	aclient.Track(a9y)
}

func (aas *AvailablilityMonitorService) Start() {
	schedule.CreateSchedule("availablilityMonitor", aas.Cron, func() {
		for _, item := range aas.Tests {
			aas.triggerTest(item)
		}
	}, aas.Logger)
}
