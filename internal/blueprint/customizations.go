package blueprint

import (
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"strconv"
	"strings"
)

type Customizations struct {
	Hostname *string                `json:"hostname,omitempty"`
	Kernel   *KernelCustomization   `json:"kernel,omitempty"`
	SSHKey   []SSHKeyCustomization  `json:"sshkey,omitempty"`
	User     []UserCustomization    `json:"user,omitempty"`
	Group    []GroupCustomization   `json:"group,omitempty"`
	Timezone *TimezoneCustomization `json:"timezone,omitempty"`
	Locale   *LocaleCustomization   `json:"locale,omitempty"`
	Firewall *FirewallCustomization `json:"firewall,omitempty"`
	Services *ServicesCustomization `json:"services,omitempty"`
}

type KernelCustomization struct {
	Append string `json:"append"`
}

type SSHKeyCustomization struct {
	User string `json:"user"`
	Key  string `json:"key"`
}

type UserCustomization struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Password    *string  `json:"password,omitempty"`
	Key         *string  `json:"key,omitempty"`
	Home        *string  `json:"home,omitempty"`
	Shell       *string  `json:"shell,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	UID         *int     `json:"uid,omitempty"`
	GID         *int     `json:"gid,omitempty"`
}

type GroupCustomization struct {
	Name string `json:"name"`
	GID  *int   `json:"gid,omitempty"`
}

type TimezoneCustomization struct {
	Timezone   *string  `json:"timezone,omitempty"`
	NTPServers []string `json:"ntpservers,omitempty"`
}

type LocaleCustomization struct {
	Languages []string `json:"languages,omitempty"`
	Keyboard  *string  `json:"keyboard,omitempty"`
}

type FirewallCustomization struct {
	Ports    []string                       `json:"ports,omitempty"`
	Services *FirewallServicesCustomization `json:"services,omitempty"`
}

type FirewallServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

type ServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

type CustomizationError struct {
	message string
}

func (e *CustomizationError) Error() string {
	return e.message
}

func (c *Customizations) customizeAll(p *pipeline.Pipeline) error {
	c.customizeHostname(p)
	c.customizeGroup(p)
	if err := c.customizeUserAndSSHKey(p); err != nil {
		return err
	}
	c.customizeTimezone(p)
	c.customizeNTPServers(p)
	c.customizeLanguages(p)
	c.customizeKeyboard(p)
	c.customizeFirewall(p)
	c.customizeServices(p)

	return nil
}

func (c *Customizations) customizeHostname(p *pipeline.Pipeline) {
	if c.Hostname == nil {
		return
	}

	p.AddStage(
		pipeline.NewHostnameStage(
			&pipeline.HostnameStageOptions{Hostname: *c.Hostname},
		),
	)
}

func (c *Customizations) customizeGroup(p *pipeline.Pipeline) {
	if len(c.Group) == 0 {
		return
	}

	// TODO: lorax-composer doesn't create group with the same name as user if both specified
	groups := make(map[string]pipeline.GroupsStageOptionsGroup)
	for _, group := range c.Group {
		groupData := pipeline.GroupsStageOptionsGroup{}
		if group.GID != nil {
			gid := strconv.Itoa(*group.GID)
			groupData.GID = &gid
		}

		groups[group.Name] = groupData
	}

	p.AddStage(
		pipeline.NewGroupsStage(
			&pipeline.GroupsStageOptions{Groups: groups},
		),
	)
}

func (c *Customizations) assertAllUsersExistForSSHCustomizations() error {
	for _, sshkey := range c.SSHKey {
		userFound := false
		for _, user := range c.User {
			if user.Name == sshkey.User {
				userFound = true
			}
		}

		if !userFound {
			return &CustomizationError{"Cannot set SSH key for non-existing user " + sshkey.User}
		}
	}
	return nil
}

func (c *Customizations) customizeUserAndSSHKey(p *pipeline.Pipeline) error {
	if len(c.User) == 0 {
		if len(c.SSHKey) > 0 {
			return &CustomizationError{"SSH key customization defined but no user customizations are defined"}
		}

		return nil
	}

	// return error if ssh key customization without user defined in user customization if found
	if e := c.assertAllUsersExistForSSHCustomizations(); e != nil {
		return e
	}

	users := make(map[string]pipeline.UsersStageOptionsUser)
	for _, user := range c.User {
		// TODO: only hashed password are currently supported as an input
		// plain-text passwords should be also supported due to parity with lorax-composer
		userData := pipeline.UsersStageOptionsUser{
			Groups:      user.Groups,
			Description: user.Description,
			Home:        user.Home,
			Shell:       user.Shell,
			Password:    user.Password,
			Key:         user.Key,
		}

		if user.UID != nil {
			uid := strconv.Itoa(*user.UID)
			userData.UID = &uid
		}

		if user.GID != nil {
			gid := strconv.Itoa(*user.GID)
			userData.GID = &gid
		}

		// process sshkey customizations
		if additionalKeys := findKeysForUser(c.SSHKey, user.Name); len(additionalKeys) > 0 {
			joinedKeys := strings.Join(additionalKeys, "\n")

			if userData.Key != nil {
				*userData.Key += "\n" + joinedKeys
			} else {
				userData.Key = &joinedKeys
			}
		}

		users[user.Name] = userData
	}

	p.AddStage(
		pipeline.NewUsersStage(
			&pipeline.UsersStageOptions{Users: users},
		),
	)

	return nil
}

func (c *Customizations) customizeTimezone(p *pipeline.Pipeline) {
	if c.Timezone == nil || c.Timezone.Timezone == nil {
		return
	}

	// TODO: lorax (anaconda) automatically installs chrony if timeservers are defined
	// except for the case when chrony is explicitly removed from installed packages (using -chrony)
	// this is currently not supported, no checks whether chrony is installed are not performed

	p.AddStage(
		pipeline.NewTimezoneStage(&pipeline.TimezoneStageOptions{
			Zone: *c.Timezone.Timezone,
		}),
	)
}

func (c *Customizations) customizeNTPServers(p *pipeline.Pipeline) {
	if c.Timezone == nil || len(c.Timezone.NTPServers) == 0 {
		return
	}

	p.AddStage(
		pipeline.NewChronyStage(&pipeline.ChronyStageOptions{
			Timeservers: c.Timezone.NTPServers,
		}),
	)
}

func (c *Customizations) customizeLanguages(p *pipeline.Pipeline) {
	if c.Locale == nil || len(c.Locale.Languages) == 0 {
		return
	}

	// TODO: you can specify more languages in customization
	// The first one is the primary one, we can set in the locale stage, this should currently work
	// Also, ALL the listed languages are installed using langpack-* packages
	// This is currently not implemented!
	// See anaconda src: pyanaconda/payload/dnfpayload.py:772

	p.AddStage(
		pipeline.NewLocaleStage(&pipeline.LocaleStageOptions{
			Language: c.Locale.Languages[0],
		}),
	)
}

func (c *Customizations) customizeKeyboard(p *pipeline.Pipeline) {
	if c.Locale == nil || c.Locale.Keyboard == nil {
		return
	}

	p.AddStage(
		pipeline.NewKeymapStage(&pipeline.KeymapStageOptions{
			Keymap: *c.Locale.Keyboard,
		}),
	)
}

func (c *Customizations) customizeFirewall(p *pipeline.Pipeline) {
	if c.Firewall == nil {
		return
	}

	var enabledServices, disabledServices []string

	if c.Firewall.Services != nil {
		enabledServices = c.Firewall.Services.Enabled
		disabledServices = c.Firewall.Services.Disabled
	}

	p.AddStage(
		pipeline.NewFirewallStage(&pipeline.FirewallStageOptions{
			Ports:            c.Firewall.Ports,
			EnabledServices:  enabledServices,
			DisabledServices: disabledServices,
		}),
	)
}

func (c *Customizations) customizeServices(p *pipeline.Pipeline) {
	if c.Services == nil {
		return
	}

	p.AddStage(
		pipeline.NewSystemdStage(&pipeline.SystemdStageOptions{
			EnabledServices:  c.Services.Enabled,
			DisabledServices: c.Services.Disabled,
		}),
	)
}

func findKeysForUser(sshKeyCustomizations []SSHKeyCustomization, user string) (keys []string) {
	for _, sshKey := range sshKeyCustomizations {
		if sshKey.User == user {
			keys = append(keys, sshKey.Key)
		}
	}
	return
}
