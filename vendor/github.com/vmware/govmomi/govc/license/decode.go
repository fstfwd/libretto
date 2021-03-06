/*
Copyright (c) 2015 VMware, Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package license

import (
	"flag"

	"github.com/vmware/govmomi/govc/cli"
	"github.com/vmware/govmomi/govc/flags"
	"github.com/vmware/govmomi/license"
	"golang.org/x/net/context"
)

type decode struct {
	*flags.ClientFlag
	*flags.OutputFlag

	feature string
}

func init() {
	cli.Register("license.decode", &decode{})
}

func (cmd *decode) Register(f *flag.FlagSet) {
	f.StringVar(&cmd.feature, "feature", "", featureUsage)
}

func (cmd *decode) Process() error { return nil }

func (cmd *decode) Usage() string {
	return "KEY..."
}

func (cmd *decode) Run(f *flag.FlagSet) error {
	client, err := cmd.Client()
	if err != nil {
		return err
	}

	var result license.InfoList
	m := license.NewManager(client)
	for _, v := range f.Args() {
		license, err := m.Decode(context.TODO(), v)
		if err != nil {
			return err
		}

		result = append(result, license)
	}

	if cmd.feature != "" {
		result = result.WithFeature(cmd.feature)
	}

	return cmd.WriteResult(licenseOutput(result))
}
