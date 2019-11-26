package fedora30

import (
	"log"
	"strconv"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
)

func customizeAll(p *pipeline.Pipeline, c *blueprint.Customizations) error {
	if c == nil {
		c = &blueprint.Customizations{}
	}
	customizeHostname(p, c)
	customizeGroup(p, c)
	if err := customizeUserAndSSHKey(p, c); err != nil {
		return err
	}
	customizeTimezone(p, c)
	customizeNTPServers(p, c)
	customizeLocale(p, c)
	customizeKeyboard(p, c)

	return nil
}

func customizeHostname(p *pipeline.Pipeline, c *blueprint.Customizations) {
	if c.Hostname == nil {
		return
	}

	p.AddStage(
		pipeline.NewHostnameStage(
			&pipeline.HostnameStageOptions{Hostname: *c.Hostname},
		),
	)
}

func customizeGroup(p *pipeline.Pipeline, c *blueprint.Customizations) {
	if len(c.Group) == 0 {
		return
	}

	groups := make(map[string]pipeline.GroupsStageOptionsGroup)
	for _, group := range c.Group {
		if userCustomizationsContainUsername(c.User, group.Name) {
			log.Println("group with name ", group.Name, " was not created, because user with same name was defined!")
			continue
		}

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

func assertAllUsersExistForSSHCustomizations(c *blueprint.Customizations) error {
	for _, sshkey := range c.SSHKey {
		userFound := false
		for _, user := range c.User {
			if user.Name == sshkey.User {
				userFound = true
			}
		}

		if !userFound {
			return &blueprint.CustomizationError{"Cannot set SSH key for non-existing user " + sshkey.User}
		}
	}
	return nil
}

func customizeUserAndSSHKey(p *pipeline.Pipeline, c *blueprint.Customizations) error {
	if len(c.User) == 0 {
		if len(c.SSHKey) > 0 {
			return &blueprint.CustomizationError{"SSH key customization defined but no user customizations are defined"}
		}

		return nil
	}

	// return error if ssh key customization without user defined in user customization if found
	if e := assertAllUsersExistForSSHCustomizations(c); e != nil {
		return e
	}

	users := make(map[string]pipeline.UsersStageOptionsUser)
	for _, user := range c.User {

		if user.Password != nil && !crypt.PasswordIsCrypted(*user.Password) {
			cryptedPassword, err := crypt.CryptSHA512(*user.Password)
			if err != nil {
				return err
			}

			user.Password = &cryptedPassword
		}

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

func customizeTimezone(p *pipeline.Pipeline, c *blueprint.Customizations) {
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

func customizeNTPServers(p *pipeline.Pipeline, c *blueprint.Customizations) {
	if c.Timezone == nil || len(c.Timezone.NTPServers) == 0 {
		return
	}

	p.AddStage(
		pipeline.NewChronyStage(&pipeline.ChronyStageOptions{
			Timeservers: c.Timezone.NTPServers,
		}),
	)
}

func customizeLocale(p *pipeline.Pipeline, c *blueprint.Customizations) {
	locale := c.Locale
	if locale == nil {
		locale = &blueprint.LocaleCustomization{
			Languages: []string{
				"en_US",
			},
		}
	}

	// TODO: you can specify more languages in customization
	// The first one is the primary one, we can set in the locale stage, this should currently work
	// Also, ALL the listed languages are installed using langpack-* packages
	// This is currently not implemented!
	// See anaconda src: pyanaconda/payload/dnfpayload.py:772

	p.AddStage(
		pipeline.NewLocaleStage(&pipeline.LocaleStageOptions{
			Language: locale.Languages[0],
		}),
	)
}

func customizeKeyboard(p *pipeline.Pipeline, c *blueprint.Customizations) {
	if c.Locale == nil || c.Locale.Keyboard == nil {
		return
	}

	p.AddStage(
		pipeline.NewKeymapStage(&pipeline.KeymapStageOptions{
			Keymap: *c.Locale.Keyboard,
		}),
	)
}

func findKeysForUser(sshKeyCustomizations []blueprint.SSHKeyCustomization, user string) (keys []string) {
	for _, sshKey := range sshKeyCustomizations {
		if sshKey.User == user {
			keys = append(keys, sshKey.Key)
		}
	}
	return
}

func userCustomizationsContainUsername(userCustomizations []blueprint.UserCustomization, name string) bool {
	for _, usr := range userCustomizations {
		if usr.Name == name {
			return true
		}
	}

	return false
}
