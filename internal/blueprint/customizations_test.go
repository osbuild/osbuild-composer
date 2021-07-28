package blueprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	}

	TestCustomizations := Customizations{
		Services: &expectedServices,
	}

	retServices := TestCustomizations.GetServices()

	assert.ElementsMatch(t, expectedServices.Enabled, retServices.Enabled)
	assert.ElementsMatch(t, expectedServices.Disabled, retServices.Disabled)
}

func TestError(t *testing.T) {
	expectedError := CustomizationError{
		Message: "test error",
	}

	retError := expectedError.Error()

	assert.Equal(t, expectedError.Message, retError)

}

//This tests calling all the functions on a Blueprint with no Customizations
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

//This tests additional scenarios where GetPrimaryLocale() returns nil values
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

//This tests additional scenario where GetTimezoneSEtting() returns nil values
func TestNilGetTimezoneSettings(t *testing.T) {

	TestCustomizationsEmpty := Customizations{}

	retTimezone, retNTPServers := TestCustomizationsEmpty.GetTimezoneSettings()

	assert.Nil(t, retTimezone)
	assert.Nil(t, retNTPServers)
}

func TestGetFilesystems(t *testing.T) {

	expectedFilesystems := []FilesystemCustomization{
		{
			MinSize:    1024,
			Mountpoint: "/",
		},
	}

	TestCustomizations := Customizations{
		Filesystem: expectedFilesystems,
	}

	retFilesystems := TestCustomizations.GetFilesystems()

	assert.ElementsMatch(t, expectedFilesystems, retFilesystems)
}

func TestGetFilesystemsMinSize(t *testing.T) {

	expectedFilesystems := []FilesystemCustomization{
		{
			MinSize:    1024,
			Mountpoint: "/",
		},
		{
			MinSize:    4096,
			Mountpoint: "/var",
		},
	}

	TestCustomizations := Customizations{
		Filesystem: expectedFilesystems,
	}

	retFilesystemsSize := TestCustomizations.GetFilesystemsMinSize()

	assert.EqualValues(t, uint64(5120), retFilesystemsSize)
}

func TestGetFilesystemsMinSizeNonSectorSize(t *testing.T) {

	expectedFilesystems := []FilesystemCustomization{
		{
			MinSize:    1025,
			Mountpoint: "/",
		},
		{
			MinSize:    4097,
			Mountpoint: "/var",
		},
	}

	TestCustomizations := Customizations{
		Filesystem: expectedFilesystems,
	}

	retFilesystemsSize := TestCustomizations.GetFilesystemsMinSize()

	assert.EqualValues(t, uint64(5632), retFilesystemsSize)
}
