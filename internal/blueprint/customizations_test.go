package blueprint

import (
	"testing"

	"github.com/osbuild/images/pkg/disk"
	"github.com/stretchr/testify/assert"
)

func TestCheckAllowed(t *testing.T) {
	Desc := "Test descritpion"
	Pass := "testpass"
	Key := "testkey"
	Home := "Home"
	Shell := "Shell"
	Groups := []string{
		"Group",
	}
	UID := 123
	GID := 321

	expectedUsers := []UserCustomization{
		UserCustomization{
			Name:        "John",
			Description: &Desc,
			Password:    &Pass,
			Key:         &Key,
			Home:        &Home,
			Shell:       &Shell,
			Groups:      Groups,
			UID:         &UID,
			GID:         &GID,
		},
	}

	var expectedHostname = "Hostname"

	x := Customizations{Hostname: &expectedHostname, User: expectedUsers}

	err := x.CheckAllowed("Hostname", "User")
	assert.NoError(t, err)

	// "User" not allowed anymore
	err = x.CheckAllowed("Hostname")
	assert.Error(t, err)

	// "Hostname" not allowed anymore
	err = x.CheckAllowed("User")
	assert.Error(t, err)
}

func TestGetHostname(t *testing.T) {

	var expectedHostname = "Hostname"

	TestCustomizations := Customizations{
		Hostname: &expectedHostname,
	}

	retHostname := TestCustomizations.GetHostname()
	assert.Equal(t, &expectedHostname, retHostname)

}

func TestGetKernel(t *testing.T) {

	expectedKernel := KernelCustomization{
		Append: "--test",
		Name:   "kernel",
	}

	TestCustomizations := Customizations{
		Kernel: &expectedKernel,
	}

	retKernel := TestCustomizations.GetKernel()

	assert.Equal(t, &expectedKernel, retKernel)
}

func TestSSHKey(t *testing.T) {

	expectedSSHKeys := []SSHKeyCustomization{
		SSHKeyCustomization{
			User: "test-user",
			Key:  "test-key",
		},
	}
	TestCustomizations := Customizations{
		SSHKey: expectedSSHKeys,
	}

	retUser := TestCustomizations.GetUsers()[0].Name
	retKey := *TestCustomizations.GetUsers()[0].Key

	assert.Equal(t, expectedSSHKeys[0].User, retUser)
	assert.Equal(t, expectedSSHKeys[0].Key, retKey)

}

func TestGetUsers(t *testing.T) {

	Desc := "Test descritpion"
	Pass := "testpass"
	Key := "testkey"
	Home := "Home"
	Shell := "Shell"
	Groups := []string{
		"Group",
	}
	UID := 123
	GID := 321
	ExpireDate := 12345

	expectedUsers := []UserCustomization{
		UserCustomization{
			Name:        "John",
			Description: &Desc,
			Password:    &Pass,
			Key:         &Key,
			Home:        &Home,
			Shell:       &Shell,
			Groups:      Groups,
			UID:         &UID,
			GID:         &GID,
			ExpireDate:  &ExpireDate,
		},
	}

	TestCustomizations := Customizations{
		User: expectedUsers,
	}

	retUsers := TestCustomizations.GetUsers()

	assert.ElementsMatch(t, expectedUsers, retUsers)
}

func TestGetGroups(t *testing.T) {

	GID := 1234
	expectedGroups := []GroupCustomization{
		GroupCustomization{
			Name: "TestGroup",
			GID:  &GID,
		},
	}

	TestCustomizations := Customizations{
		Group: expectedGroups,
	}

	retGroups := TestCustomizations.GetGroups()

	assert.ElementsMatch(t, expectedGroups, retGroups)
}

func TestGetTimezoneSettings(t *testing.T) {

	expectedTimezone := "testZONE"
	expectedNTPServers := []string{
		"server",
	}

	expectedTimezoneCustomization := TimezoneCustomization{
		Timezone:   &expectedTimezone,
		NTPServers: expectedNTPServers,
	}

	TestCustomizations := Customizations{
		Timezone: &expectedTimezoneCustomization,
	}

	retTimezone, retNTPServers := TestCustomizations.GetTimezoneSettings()

	assert.Equal(t, expectedTimezone, *retTimezone)
	assert.Equal(t, expectedNTPServers, retNTPServers)

}

func TestGetPrimaryLocale(t *testing.T) {

	expectedLanguages := []string{
		"enUS",
	}
	expectedKeyboard := "en"

	expectedLocaleCustomization := LocaleCustomization{
		Languages: expectedLanguages,
		Keyboard:  &expectedKeyboard,
	}

	TestCustomizations := Customizations{
		Locale: &expectedLocaleCustomization,
	}

	retLanguage, retKeyboard := TestCustomizations.GetPrimaryLocale()

	assert.Equal(t, expectedLanguages[0], *retLanguage)
	assert.Equal(t, expectedKeyboard, *retKeyboard)
}

func TestGetFirewall(t *testing.T) {

	expectedPorts := []string{"22", "9090"}

	expectedServices := FirewallServicesCustomization{
		Enabled:  []string{"cockpit", "osbuild-composer"},
		Disabled: []string{"TCP", "httpd"},
	}

	expectedFirewall := FirewallCustomization{
		Ports:    expectedPorts,
		Services: &expectedServices,
	}

	TestCustomizations := Customizations{
		Firewall: &expectedFirewall,
	}

	retFirewall := TestCustomizations.GetFirewall()

	assert.ElementsMatch(t, expectedFirewall.Ports, retFirewall.Ports)
	assert.ElementsMatch(t, expectedFirewall.Services.Enabled, retFirewall.Services.Enabled)
	assert.ElementsMatch(t, expectedFirewall.Services.Disabled, retFirewall.Services.Disabled)
}

func TestGetServices(t *testing.T) {

	expectedServices := ServicesCustomization{
		Enabled:  []string{"cockpit", "osbuild-composer"},
		Disabled: []string{"sshd", "ftp"},
		Masked:   []string{"firewalld"},
	}

	TestCustomizations := Customizations{
		Services: &expectedServices,
	}

	retServices := TestCustomizations.GetServices()

	assert.ElementsMatch(t, expectedServices.Enabled, retServices.Enabled)
	assert.ElementsMatch(t, expectedServices.Disabled, retServices.Disabled)
	assert.ElementsMatch(t, expectedServices.Masked, retServices.Masked)
}

func TestError(t *testing.T) {
	expectedError := CustomizationError{
		Message: "test error",
	}

	retError := expectedError.Error()

	assert.Equal(t, expectedError.Message, retError)

}

// This tests calling all the functions on a Blueprint with no Customizations
func TestNoCustomizationsInBlueprint(t *testing.T) {

	TestBP := Blueprint{}

	assert.Nil(t, TestBP.Customizations.GetHostname())
	assert.Nil(t, TestBP.Customizations.GetUsers())
	assert.Nil(t, TestBP.Customizations.GetGroups())
	assert.Equal(t, &KernelCustomization{Name: "kernel"}, TestBP.Customizations.GetKernel())
	assert.Nil(t, TestBP.Customizations.GetFirewall())
	assert.Nil(t, TestBP.Customizations.GetServices())

	nilLanguage, nilKeyboard := TestBP.Customizations.GetPrimaryLocale()
	assert.Nil(t, nilLanguage)
	assert.Nil(t, nilKeyboard)

	nilTimezone, nilNTPServers := TestBP.Customizations.GetTimezoneSettings()
	assert.Nil(t, nilTimezone)
	assert.Nil(t, nilNTPServers)
}

// This tests additional scenarios where GetPrimaryLocale() returns nil values
func TestNilGetPrimaryLocale(t *testing.T) {

	//Case empty Customization
	TestCustomizationsEmpty := Customizations{}

	retLanguage, retKeyboard := TestCustomizationsEmpty.GetPrimaryLocale()

	assert.Nil(t, retLanguage)
	assert.Nil(t, retKeyboard)

	//Case empty Languages
	expectedKeyboard := "en"
	expectedLocaleCustomization := LocaleCustomization{
		Keyboard: &expectedKeyboard,
	}

	TestCustomizations := Customizations{
		Locale: &expectedLocaleCustomization,
	}

	retLanguage, retKeyboard = TestCustomizations.GetPrimaryLocale()

	assert.Nil(t, retLanguage)
	assert.Equal(t, expectedKeyboard, *retKeyboard)

}

// This tests additional scenario where GetTimezoneSEtting() returns nil values
func TestNilGetTimezoneSettings(t *testing.T) {

	TestCustomizationsEmpty := Customizations{}

	retTimezone, retNTPServers := TestCustomizationsEmpty.GetTimezoneSettings()

	assert.Nil(t, retTimezone)
	assert.Nil(t, retNTPServers)
}

func TestGetOpenSCAPConfig(t *testing.T) {

	expectedOscap := OpenSCAPCustomization{
		DataStream: "test-data-stream.xml",
		ProfileID:  "test_profile",
	}

	TestCustomizations := Customizations{
		OpenSCAP: &expectedOscap,
	}

	retOpenSCAPCustomiztions := TestCustomizations.GetOpenSCAP()

	assert.EqualValues(t, expectedOscap, *retOpenSCAPCustomiztions)
}

func TestGetPartitioningMode(t *testing.T) {
	// No customizations returns Default which is actually AutoLVM,
	// but that is handled by the images code
	var c *Customizations
	pm, err := c.GetPartitioningMode()
	assert.NoError(t, err)
	assert.Equal(t, disk.DefaultPartitioningMode, pm)

	// Empty defaults to Default which is actually AutoLVM,
	// but that is handled by the images code
	c = &Customizations{}
	_, err = c.GetPartitioningMode()
	assert.NoError(t, err)
	assert.Equal(t, disk.DefaultPartitioningMode, pm)

	// Unknown mode returns an error
	c = &Customizations{
		PartitioningMode: "all-of-them",
	}
	_, err = c.GetPartitioningMode()
	assert.Error(t, err)

	// And a known mode returns the correct type
	c = &Customizations{
		PartitioningMode: "lvm",
	}
	pm, err = c.GetPartitioningMode()
	assert.NoError(t, err)
	assert.Equal(t, disk.LVMPartitioningMode, pm)
}
