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

package vapp

import (
	"flag"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/govc/cli"
	"github.com/vmware/govmomi/govc/flags"
	"golang.org/x/net/context"
)

type destroy struct {
	*flags.DatacenterFlag
}

func init() {
	cli.Register("vapp.destroy", &destroy{})
}

func (cmd *destroy) Register(f *flag.FlagSet) {}

func (cmd *destroy) Process() error { return nil }

func (cmd *destroy) Usage() string {
	return "VAPP..."
}

func (cmd *destroy) Run(f *flag.FlagSet) error {
	if f.NArg() == 0 {
		return flag.ErrHelp
	}

	finder, err := cmd.Finder()
	if err != nil {
		return err
	}

	for _, arg := range f.Args() {
		vapps, err := finder.VirtualAppList(context.TODO(), arg)
		if err != nil {
			if _, ok := err.(*find.NotFoundError); ok {
				// Ignore if vapp cannot be found
				continue
			}

			return err
		}

		for _, vapp := range vapps {
			task, err := vapp.PowerOffVApp_Task(context.TODO(), false)
			if err != nil {
				return err
			}
			err = task.Wait(context.TODO())
			if err != nil {
				return err
			}

			task, err = vapp.Destroy(context.TODO())
			if err != nil {
				return err
			}
			err = task.Wait(context.TODO())
			if err != nil {
				return err
			}
		}
	}

	return nil
}
