package pathpolicy

// MountpointPolicies is a set of default mountpoint policies used for filesystem customizations
var MountpointPolicies = NewPathPolicies(map[string]PathPolicy{
	"/":     {Exact: true},
	"/boot": {Exact: true},
	"/var":  {},
	"/opt":  {},
	"/srv":  {},
	"/usr":  {},
	"/app":  {},
	"/data": {},
	"/home": {},
	"/tmp":  {},
})

// CustomDirectoriesPolicies is a set of default policies for custom directories
var CustomDirectoriesPolicies = NewPathPolicies(map[string]PathPolicy{
	"/":    {Deny: true},
	"/etc": {},
})

// CustomFilesPolicies is a set of default policies for custom files
var CustomFilesPolicies = NewPathPolicies(map[string]PathPolicy{
	"/":           {Deny: true},
	"/etc":        {},
	"/etc/fstab":  {Deny: true},
	"/etc/shadow": {Deny: true},
	"/etc/passwd": {Deny: true},
	"/etc/group":  {Deny: true},
})
