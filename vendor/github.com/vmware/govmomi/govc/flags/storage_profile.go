/*
Copyright (c) 2024-2024 VMware, Inc. All Rights Reserved.

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

package flags

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/vmware/govmomi/vim25/types"
)

type StorageProfileFlag struct {
	*ClientFlag

	Name []string

	option string
}

func NewStorageProfileFlag(ctx context.Context, option ...string) (*StorageProfileFlag, context.Context) {
	v := &StorageProfileFlag{}
	if len(option) == 1 {
		v.option = option[0]
	} else {
		v.option = "profile"
	}
	v.ClientFlag, ctx = NewClientFlag(ctx)
	return v, ctx
}

func (e *StorageProfileFlag) String() string {
	return fmt.Sprint(e.Name)
}

func (e *StorageProfileFlag) Set(value string) error {
	e.Name = append(e.Name, value)
	return nil
}

func (flag *StorageProfileFlag) Register(ctx context.Context, f *flag.FlagSet) {
	flag.ClientFlag.Register(ctx, f)

	f.Var(flag, flag.option, "Storage profile name or ID")
}

func (flag *StorageProfileFlag) StorageProfileList(ctx context.Context) ([]string, error) {
	if len(flag.Name) == 0 {
		return nil, nil
	}

	c, err := flag.PbmClient()
	if err != nil {
		return nil, err
	}
	m, err := c.ProfileMap(ctx)
	if err != nil {
		return nil, err
	}

	list := make([]string, len(flag.Name))

	for i, name := range flag.Name {
		p, ok := m.Name[name]
		if !ok {
			return nil, fmt.Errorf("storage profile %q not found", name)
		}

		list[i] = p.GetPbmProfile().ProfileId.UniqueId
	}

	return list, nil
}

func (flag *StorageProfileFlag) StorageProfile(ctx context.Context) (string, error) {
	switch len(flag.Name) {
	case 0:
		return "", nil
	case 1:
	default:
		return "", errors.New("only 1 '-profile' can be specified")
	}

	list, err := flag.StorageProfileList(ctx)
	if err != nil {
		return "", err
	}

	return list[0], nil
}

func (flag *StorageProfileFlag) StorageProfileSpec(ctx context.Context) ([]types.BaseVirtualMachineProfileSpec, error) {
	if len(flag.Name) == 0 {
		return nil, nil
	}

	list, err := flag.StorageProfileList(ctx)
	if err != nil {
		return nil, err
	}

	spec := make([]types.BaseVirtualMachineProfileSpec, len(list))
	for i, name := range list {
		spec[i] = &types.VirtualMachineDefinedProfileSpec{
			ProfileId: name,
		}
	}
	return spec, nil
}
