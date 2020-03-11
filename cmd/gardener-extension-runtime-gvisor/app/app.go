// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"github.com/gardener/gardener-extension-runtime-gvisor/pkg/gvisor"
	"github.com/gardener/gardener-extension-runtime-gvisor/pkg/healthcheck"
	"os"
	"github.com/gardener/gardener-extension-runtime-gvisor/pkg/gvisor"
	gvisorcmd "github.com/gardener/gardener-extension-runtime-gvisor/pkg/cmd"

	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener-extensions/pkg/controller/cmd"
	"github.com/gardener/gardener-extensions/pkg/util"

	"github.com/spf13/cobra"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewControllerCommand creates a new command that is used to start the Container runtime gvisor controller.
func NewControllerCommand(ctx context.Context) *cobra.Command {

	var (
		restOpts = &controllercmd.RESTOptions{}
		mgrOpts  = &controllercmd.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        controllercmd.LeaderElectionNameID(gvisor.Name),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
		}
		// options for the networking-calico controller
		calicoCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		reconcileOpts = &controllercmd.ReconcilerOptions{
			IgnoreOperationAnnotation: true,
		}

		// options for the health care controller
		healthCheckCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		configFileOpts = &gvisorcmd.ConfigOptions{}

		aggOption = controllercmd.NewOptionAggregator(
			restOpts,
			mgrOpts,
			calicoCtrlOpts,
			controllercmd.PrefixOption("healthcheck-", healthCheckCtrlOpts),
			reconcileOpts,
			configFileOpts,
		)
	)



	options := NewOptions()

	cmd := &cobra.Command{
		Use:   "runtime-gvisor-controller-manager",
		Short: "Container runtime gvisor",

		Run: func(cmd *cobra.Command, args []string) {
			if err := options.optionAggregator.Complete(); err != nil {
				controllercmd.LogErrAndExit(err, "Error completing options")
			}
			options.run(ctx)
		},
	}

	options.optionAggregator.AddFlags(cmd.Flags())

	return cmd
}

func (o *Options) run(ctx context.Context) {
	util.ApplyClientConnectionConfigurationToRESTConfig(&componentbaseconfig.ClientConnectionConfiguration{
		QPS:   100.0,
		Burst: 130,
	}, o.restOptions.Completed().Config)

	mgr, err := manager.New(o.restOptions.Completed().Config, o.managerOptions.Completed().Options())
	if err != nil {
		controllercmd.LogErrAndExit(err, "Could not instantiate controller-manager")
	}

	if err := extensionscontroller.AddToScheme(mgr.GetScheme()); err != nil {
		controllercmd.LogErrAndExit(err, "Could not update manager scheme")
	}

	o.controllerOptions.Completed().Apply(&config.ControllerConfig.ControllerOptions)
	o.reconcileOptions.Completed().Apply(&config.ControllerConfig.IgnoreOperationAnnotation)

	if err := o.controllerSwitches.Completed().AddToManager(mgr); err != nil {
		controllercmd.LogErrAndExit(err, "Could not add controllers to manager")
	}

	if err := mgr.Start(ctx.Done()); err != nil {
		controllercmd.LogErrAndExit(err, "Error running manager")
	}
}

// NewControllerManagerCommand creates a new command for running a Calico controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {


	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s-controller-manager", calico.Name),

		Run: func(cmd *cobra.Command, args []string) {
			if err := aggOption.Complete(); err != nil {
				controllercmd.LogErrAndExit(err, "Error completing options")
			}
			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, restOpts.Completed().Config)

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not instantiate manager")
			}

			if err := controller.AddToScheme(mgr.GetScheme()); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}

			if err := calicoinstall.AddToScheme(mgr.GetScheme()); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}

			reconcileOpts.Completed().Apply(&calicocontroller.DefaultAddOptions.IgnoreOperationAnnotation)
			configFileOpts.Completed().ApplyHealthCheckConfig(&healthcheck.AddOptions.HealthCheckConfig)
			healthCheckCtrlOpts.Completed().Apply(&healthcheck.AddOptions.Controller)

			if err := calicocontroller.AddToManager(mgr); err != nil {
				controllercmd.LogErrAndExit(err, "Could not add controllers to manager")
			}

			if err := healthcheck.AddToManager(mgr); err != nil {
				controllercmd.LogErrAndExit(err, "Could not add health check controller to manager")
			}

			if err := mgr.Start(ctx.Done()); err != nil {
				controllercmd.LogErrAndExit(err, "Error running manager")
			}
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}