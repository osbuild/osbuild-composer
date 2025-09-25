package policies

import (
	"github.com/osbuild/images/pkg/pathpolicy"
)

// MountpointPolicies is a set of default mountpoint policies used for filesystem customizations
var MountpointPolicies = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
	"/": {},
	// /etc must be on the root filesystem
	"/etc": {Deny: true},
	// NB: any mountpoints under /usr are not supported by systemd fstab
	// generator in initram before the switch-root, so we don't allow them.
	"/usr": {Exact: true},
	// API filesystems
	"/sys":  {Deny: true},
	"/proc": {Deny: true},
	"/dev":  {Deny: true},
	"/run":  {Deny: true},
	// not allowed due to merged-usr
	"/bin":   {Deny: true},
	"/sbin":  {Deny: true},
	"/lib":   {Deny: true},
	"/lib64": {Deny: true},
	// used by ext filesystems
	"/lost+found": {Deny: true},
	// used by systemd / ostree
	"/sysroot": {Deny: true},
	// symlink to ../run which is on tmpfs
	"/var/run": {Deny: true},
	// symlink to ../run/lock which is on tmpfs
	"/var/lock": {Deny: true},
})

// CustomDirectoriesPolicies is a set of default policies for custom directories
var CustomDirectoriesPolicies = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
	"/":           {},
	"/bin":        {Deny: true},
	"/boot":       {Deny: true},
	"/dev":        {Deny: true},
	"/lib":        {Deny: true},
	"/lib64":      {Deny: true},
	"/lost+found": {Deny: true},
	"/proc":       {Deny: true},
	"/run":        {Deny: true},
	"/sbin":       {Deny: true},
	"/sys":        {Deny: true},
	"/sysroot":    {Deny: true},
	"/tmp":        {Deny: true},
	"/usr":        {Deny: true},
	"/usr/local":  {},
	"/var/run":    {Deny: true},
	"/var/tmp":    {Deny: true},
	"/efi":        {Deny: true},
})

// CustomFilesPolicies is a set of default policies for custom files
var CustomFilesPolicies = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
	"/":           {},
	"/bin":        {Deny: true},
	"/boot":       {Deny: true},
	"/dev":        {Deny: true},
	"/efi":        {Deny: true},
	"/etc/fstab":  {Deny: true},
	"/etc/group":  {Deny: true},
	"/etc/passwd": {Deny: true},
	"/etc/shadow": {Deny: true},
	"/lib":        {Deny: true},
	"/lib64":      {Deny: true},
	"/lost+found": {Deny: true},
	"/proc":       {Deny: true},
	"/run":        {Deny: true},
	"/sbin":       {Deny: true},
	"/sys":        {Deny: true},
	"/sysroot":    {Deny: true},
	"/tmp":        {Deny: true},
	"/usr":        {Deny: true},
	"/usr/local":  {},
	"/var/run":    {Deny: true},
	"/var/tmp":    {Deny: true},
})

// MountpointPolicies for ostree
var OstreeMountpointPolicies = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
	"/":             {},
	"/home":         {Deny: true}, // symlink to var/home
	"/mnt":          {Deny: true}, // symlink to var/mnt
	"/opt":          {Deny: true}, // symlink to var/opt
	"/ostree":       {Deny: true}, // symlink to sysroot/ostree
	"/root":         {Deny: true}, // symlink to var/roothome
	"/srv":          {Deny: true}, // symlink to var/srv
	"/var/home":     {Deny: true},
	"/var/mnt":      {Deny: true},
	"/var/opt":      {Deny: true},
	"/var/roothome": {Deny: true},
	"/var/srv":      {Deny: true},
	"/var/usrlocal": {Deny: true},
})

// CustomDirectoriesPolicies for ostree
var OstreeCustomDirectoriesPolicies = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
	"/":    {Deny: true},
	"/etc": {},
})

// CustomFilesPolicies for ostree
var OstreeCustomFilesPolicies = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
	"/":               {Deny: true},
	"/etc":            {},
	"/root":           {},
	"/usr/local/bin":  {},
	"/usr/local/sbin": {},
	"/etc/fstab":      {Deny: true},
	"/etc/shadow":     {Deny: true},
	"/etc/passwd":     {Deny: true},
	"/etc/group":      {Deny: true},
})
