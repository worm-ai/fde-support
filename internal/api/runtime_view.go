package api

import (
	"fde-support/internal/environment"
	"fde-support/internal/manifest"
)

const (
	chatPath = "/chat"
	webPath  = "/web/"
)

type runtimeView struct {
	Solution    string              `json:"solution"`
	Version     string              `json:"version"`
	Environment string              `json:"environment"`
	TracePath   string              `json:"tracePath"`
	ChatPath    string              `json:"chatPath"`
	WebPath     string              `json:"webPath"`
	Sensors     []runtimeSensorView `json:"sensors"`
}

type runtimeSensorView struct {
	ID           string   `json:"id"`
	EndpointPath string   `json:"endpointPath,omitempty"`
	SignalTypes  []string `json:"signalTypes"`
}

func newRuntimeView(m *manifest.SolutionManifest, env environment.ResolvedEnvironment) runtimeView {
	sensors := make([]runtimeSensorView, 0, len(m.Perception.Sensors))
	for _, sensor := range m.Perception.Sensors {
		endpointPath, _ := sensor.Config["endpointPath"].(string)
		signalTypes := append([]string(nil), sensor.SignalTypes...)
		sensors = append(sensors, runtimeSensorView{
			ID:           sensor.ID,
			EndpointPath: endpointPath,
			SignalTypes:  signalTypes,
		})
	}
	return runtimeView{
		Solution:    m.Metadata.Name,
		Version:     m.Metadata.Version,
		Environment: env.EnvironmentName,
		TracePath:   env.TracePath,
		ChatPath:    chatPath,
		WebPath:     webPath,
		Sensors:     sensors,
	}
}
