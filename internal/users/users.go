package users

type User struct {
	Name        string
	Description *string
	Password    *string
	Key         *string
	Home        *string
	Shell       *string
	Groups      []string
	UID         *int
	GID         *int
}

type Group struct {
	Name string
	GID  *int
}
