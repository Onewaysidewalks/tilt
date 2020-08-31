package metrics

import (
	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Extension struct{}

func NewExtension() Extension {
	return Extension{}
}

func (e Extension) NewState() interface{} {
	return model.MetricsSettings{
		Address: "opentelemetry.tilt.dev:443",
	}
}

func (Extension) OnStart(env *starkit.Environment) error {
	return env.AddBuiltin("experimental_metrics_settings", setMetricsSettings)
}

func setMetricsSettings(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	err := starkit.SetState(thread, func(settings model.MetricsSettings) (model.MetricsSettings, error) {
		err := starkit.UnpackArgs(thread, fn.Name(), args, kwargs,
			"enabled?", &settings.Enabled,
			"address?", &settings.Address,
			"insecure?", &settings.Insecure)
		if err != nil {
			return model.MetricsSettings{}, err
		}
		return settings, nil
	})

	return starlark.None, err
}

var _ starkit.StatefulExtension = Extension{}

func MustState(model starkit.Model) model.MetricsSettings {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (model.MetricsSettings, error) {
	var state model.MetricsSettings
	err := m.Load(&state)
	return state, err
}
