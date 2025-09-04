package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/olog"
)

type VM struct {
	ResourceGroup string `json:"resourcegroup"`
	VNet          string `json:"vnet"`
	Subnet        string `json:"subnet"`
	SG            string `json:"sg"`
	IPName        string `json:"ip-name"`
	IPAddress     string `json:"ip-address"`
	Nic           string `json:"nic"`
	Name          string `json:"name"`
	Disk          string `json:"disk"`
}

func (ac Client) CreateVM(ctx context.Context, resourceGroup, image, name, size, username, sshKey string) (*VM, error) {
	vm := VM{
		ResourceGroup: resourceGroup,
		Name:          name,
	}
	var err error
	defer func() {
		if err != nil {
			err = ac.DestroyVM(ctx, &vm)
			if err != nil {
				olog.Printf("unable to destroy vm: %s", err.Error())
			}
		}
	}()

	location, err := ac.GetResourceGroupLocation(ctx, resourceGroup)
	if err != nil {
		return nil, fmt.Errorf("retrieving resource group location failed: %w", err)
	}

	virtualNetwork, err := ac.createVirtualNetwork(ctx, resourceGroup, location, fmt.Sprintf("%s-vnet", vm.Name))
	if err != nil {
		return nil, err
	}
	vm.VNet = common.ValueOrEmpty(virtualNetwork.Name)

	subnet, err := ac.createSubnet(ctx, resourceGroup, common.ValueOrEmpty(virtualNetwork.Name), fmt.Sprintf("%s-subnet", vm.Name))
	if err != nil {
		return nil, err
	}
	vm.Subnet = common.ValueOrEmpty(subnet.Name)

	publicIP, err := ac.createPublicIP(ctx, resourceGroup, location, fmt.Sprintf("%s-ip", vm.Name))
	if err != nil {
		return nil, err
	}
	vm.IPName = common.ValueOrEmpty(publicIP.Name)
	vm.IPAddress = common.ValueOrEmpty(publicIP.Properties.IPAddress)

	sg, err := ac.createSG(ctx, resourceGroup, location, fmt.Sprintf("%s-sg", vm.Name))
	if err != nil {
		return nil, err
	}
	vm.SG = common.ValueOrEmpty(sg.Name)

	intf, err := ac.createInterface(ctx, resourceGroup, location, *subnet.ID, *publicIP.ID, *sg.ID, fmt.Sprintf("%s-intf", vm.Name))
	if err != nil {
		return nil, err
	}
	vm.Nic = common.ValueOrEmpty(intf.Name)

	virtualMachine, err := ac.createVM(ctx, resourceGroup, location, image, size, *intf.ID, fmt.Sprintf("%s-disk", vm.Name), vm.Name, username, sshKey)
	if err != nil {
		return nil, err
	}
	vm.Name = common.ValueOrEmpty(virtualMachine.Name)
	vm.Disk = common.ValueOrEmpty(virtualMachine.Properties.StorageProfile.OSDisk.Name)
	return &vm, nil
}

func (ac Client) DestroyVM(ctx context.Context, vm *VM) error {
	err := ac.deleteVirtualMachine(ctx, vm)
	if err != nil {
		return err
	}
	err = ac.deleteDisk(ctx, vm)
	if err != nil {
		return err
	}
	err = ac.deleteInterface(ctx, vm)
	if err != nil {
		return err
	}
	err = ac.deleteSG(ctx, vm)
	if err != nil {
		return err
	}
	err = ac.deletePublicIP(ctx, vm)
	if err != nil {
		return err
	}
	err = ac.deleteSubnet(ctx, vm)
	if err != nil {
		return err
	}
	err = ac.deleteVirtualNetwork(ctx, vm)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) createVirtualNetwork(ctx context.Context, resourceGroup, location, name string) (*armnetwork.VirtualNetwork, error) {
	poller, err := ac.vnets.BeginCreateOrUpdate(ctx, resourceGroup, name, armnetwork.VirtualNetwork{
		Location: &location,
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{
					common.ToPtr("10.1.0.0/16"),
				},
			},
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.VirtualNetwork, err
}

func (ac Client) deleteVirtualNetwork(ctx context.Context, vm *VM) error {
	if vm.VNet == "" {
		return nil
	}
	poller, err := ac.vnets.BeginDelete(ctx, vm.ResourceGroup, vm.VNet, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) createSubnet(ctx context.Context, resourceGroup, vnet, name string) (*armnetwork.Subnet, error) {
	poller, err := ac.subnets.BeginCreateOrUpdate(ctx, resourceGroup, vnet, name, armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: common.ToPtr("10.1.10.0/24"),
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.Subnet, nil
}

func (ac Client) deleteSubnet(ctx context.Context, vm *VM) error {
	if vm.Subnet == "" {
		return nil
	}
	poller, err := ac.subnets.BeginDelete(ctx, vm.ResourceGroup, vm.VNet, vm.Subnet, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) createSG(ctx context.Context, resourceGroup, location, name string) (*armnetwork.SecurityGroup, error) {
	poller, err := ac.securityGroups.BeginCreateOrUpdate(ctx, resourceGroup, name, armnetwork.SecurityGroup{
		Location: &location,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: common.ToPtr("ssh"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						SourceAddressPrefix:      common.ToPtr("*"),
						SourcePortRange:          common.ToPtr("*"),
						DestinationAddressPrefix: common.ToPtr("*"),
						DestinationPortRange:     common.ToPtr("22"),
						Protocol:                 common.ToPtr(armnetwork.SecurityRuleProtocolTCP),
						Access:                   common.ToPtr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 common.ToPtr[int32](100),
						Description:              common.ToPtr("ssh"),
						Direction:                common.ToPtr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.SecurityGroup, nil
}

func (ac Client) deleteSG(ctx context.Context, vm *VM) error {
	if vm.SG == "" {
		return nil
	}
	poller, err := ac.securityGroups.BeginDelete(ctx, vm.ResourceGroup, vm.SG, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) createPublicIP(ctx context.Context, resourceGroup, location, name string) (*armnetwork.PublicIPAddress, error) {
	poller, err := ac.publicIPs.BeginCreateOrUpdate(ctx, resourceGroup, name, armnetwork.PublicIPAddress{
		Location: common.ToPtr(location),
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: common.ToPtr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.PublicIPAddress, err
}

func (ac Client) deletePublicIP(ctx context.Context, vm *VM) error {
	if vm.IPName == "" {
		return nil
	}
	poller, err := ac.publicIPs.BeginDelete(ctx, vm.ResourceGroup, vm.IPName, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) createInterface(ctx context.Context, resourceGroup, location, subnet, publicIP, securityGroup, name string) (*armnetwork.Interface, error) {
	poller, err := ac.interfaces.BeginCreateOrUpdate(ctx, resourceGroup, name, armnetwork.Interface{
		Location: common.ToPtr(location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: common.ToPtr("ipConfig"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: common.ToPtr(armnetwork.IPAllocationMethodDynamic),
						Subnet: &armnetwork.Subnet{
							ID: common.ToPtr(subnet),
						},
						PublicIPAddress: &armnetwork.PublicIPAddress{
							ID: common.ToPtr(publicIP),
						},
					},
				},
			},
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: common.ToPtr(securityGroup),
			},
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.Interface, err
}

func (ac Client) deleteInterface(ctx context.Context, vm *VM) error {
	if vm.Nic == "" {
		return nil
	}
	poller, err := ac.interfaces.BeginDelete(ctx, vm.ResourceGroup, vm.Nic, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

func (ac Client) createVM(ctx context.Context, resourceGroup, location, image, size, nic, diskName, name, username, sshKey string) (*armcompute.VirtualMachine, error) {
	vm := armcompute.VirtualMachine{
		Location: common.ToPtr(location),
		Identity: &armcompute.VirtualMachineIdentity{
			Type: common.ToPtr(armcompute.ResourceIdentityTypeNone),
		},
		Properties: &armcompute.VirtualMachineProperties{
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					ID: common.ToPtr(image),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         common.ToPtr(diskName),
					CreateOption: common.ToPtr(armcompute.DiskCreateOptionTypesFromImage),
					Caching:      common.ToPtr(armcompute.CachingTypesReadWrite),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: common.ToPtr(armcompute.StorageAccountTypesStandardLRS),
					},
				},
			},
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: common.ToPtr(armcompute.VirtualMachineSizeTypes(size)),
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  common.ToPtr(name),
				AdminUsername: common.ToPtr(username),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: common.ToPtr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    common.ToPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", username)),
								KeyData: common.ToPtr(sshKey),
							},
						},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: common.ToPtr(nic),
					},
				},
			},
		},
	}

	poller, err := ac.vms.BeginCreateOrUpdate(ctx, resourceGroup, name, vm, nil)
	if err != nil {
		return nil, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &resp.VirtualMachine, nil
}

func (ac Client) deleteVirtualMachine(ctx context.Context, vm *VM) error {
	if vm.Name == "" {
		return nil
	}
	poller, err := ac.vms.BeginDelete(ctx, vm.ResourceGroup, vm.Name, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) deleteDisk(ctx context.Context, vm *VM) error {
	if vm.Disk == "" {
		return nil
	}
	poller, err := ac.disks.BeginDelete(ctx, vm.ResourceGroup, vm.Disk, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}
