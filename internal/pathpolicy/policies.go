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
