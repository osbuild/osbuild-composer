package osbuild

import (
	"strings"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUsersStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.users",
		Options: &UsersStageOptions{},
	}
	actualStage := NewUsersStage(&UsersStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewUsersStageOptionsPassword(t *testing.T) {
	Pass := "testpass"
	EmptyPass := ""
	CryptPass := "$6$RWdHzrPfoM6BMuIP$gKYlBXQuJgP.G2j2twbOyxYjFDPUQw8Jp.gWe1WD/obX0RMyfgw5vt.Mn/tLLX4mQjaklSiIzoAW3HrVQRg4Q." // #nosec G101

	users := []users.User{
		{
			Name:     "bart",
			Password: &Pass,
		},
		{
			Name:     "lisa",
			Password: &CryptPass,
		},
		{
			Name:     "maggie",
			Password: &EmptyPass,
		},
		{
			Name: "homer",
		},
	}

	options, err := NewUsersStageOptions(users, false)
	require.Nil(t, err)
	require.NotNil(t, options)

	// bart's password should now be a hash
	assert.True(t, strings.HasPrefix(*options.Users["bart"].Password, "$6$"))

	// lisa's password should be left alone (already hashed)
	assert.Equal(t, CryptPass, *options.Users["lisa"].Password)

	// maggie's password should now be nil (locked account)
	assert.Nil(t, options.Users["maggie"].Password)

	// homer's password should still be nil (locked account)
	assert.Nil(t, options.Users["homer"].Password)
}
