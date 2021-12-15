package v1

import (
	"github.com/mudler/yip/pkg/executor"
	"github.com/mudler/yip/pkg/plugins"
)

// CloudInitRunner returns a default cloud init executor with the Elemental plugin set.
// It accepts a logger which is used inside the runner.
func CloudInitRunner(l Logger) executor.Executor {
	return executor.NewExecutor(
		executor.WithConditionals(
			plugins.NodeConditional,
			plugins.IfConditional,
		),
		executor.WithLogger(l),
		executor.WithPlugins(
			// Note, the plugin execution order depends on the order passed here
			plugins.DNS,
			plugins.Download,
			plugins.Git,
			plugins.Entities,
			plugins.EnsureDirectories,
			plugins.EnsureFiles,
			plugins.Commands,
			plugins.DeleteEntities,
			plugins.Hostname,
			plugins.Sysctl,
			plugins.SSH,
			plugins.User,
			plugins.LoadModules,
			plugins.Timesyncd,
			plugins.Systemctl,
			plugins.Environment,
			plugins.SystemdFirstboot,
			plugins.DataSources,
			plugins.Layout,
		),
	)
}
