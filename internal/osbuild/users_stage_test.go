package osbuild

import (
	"strings"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"

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

	users := []blueprint.UserCustomization{
		blueprint.UserCustomization{
			Name:     "bart",
			Password: &Pass,
		},
		blueprint.UserCustomization{
			Name:     "lisa",
			Password: &CryptPass,
		},
		blueprint.UserCustomization{
			Name:     "maggie",
			Password: &EmptyPass,
		},
		blueprint.UserCustomization{
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
