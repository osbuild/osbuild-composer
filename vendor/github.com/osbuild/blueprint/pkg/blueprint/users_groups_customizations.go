package blueprint

import (
	"errors"
	"fmt"
	"strings"
)

func (c *Customizations) GetUsers() []UserCustomization {
	if c == nil || (c.User == nil && c.SSHKey == nil) {
		return nil
	}

	var users []UserCustomization

	// prepend sshkey for backwards compat (overridden by users)
	if len(c.SSHKey) > 0 {
		for _, k := range c.SSHKey {
			key := k.Key
			users = append(users, UserCustomization{
				Name: k.User,
				Key:  &key,
			})
		}
	}

	users = append(users, c.User...)

	// sanitize user home directory in blueprint: if it has a trailing slash,
	// it might lead to the directory not getting the correct selinux labels
	for idx := range users {
		u := users[idx]
		if u.Home != nil {
			homedir := strings.TrimRight(*u.Home, "/")
			u.Home = &homedir
			users[idx] = u
		}
	}
	return users
}

type GroupsCustomization []GroupCustomization

func (g GroupsCustomization) Validate() error {
	names := make(map[string]bool)
	gids := make(map[int]bool)

	errs := make([]error, 0)

	for _, group := range g {
		if names[group.Name] {
			errs = append(errs, fmt.Errorf("duplicate group name: %s", group.Name))
		}
		names[group.Name] = true

		if group.GID != nil {
			if gids[*group.GID] {
				errs = append(errs, fmt.Errorf("duplicate group ID: %d", *group.GID))
			}
			gids[*group.GID] = true
		}
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("invalid group customizations:\n%w", err)
	}

	return nil
}

func (c *Customizations) GetGroups() (GroupsCustomization, error) {
	if c == nil {
		return nil, nil
	}

	if err := c.Group.Validate(); err != nil {
		return nil, err
	}

	return c.Group, nil
}
