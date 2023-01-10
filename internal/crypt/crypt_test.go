//go:build !darwin
// +build !darwin

package crypt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_crypt_PasswordIsCrypted(t *testing.T) {

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{
			name:     "bcrypt",
			password: "$2b$04$123465789012345678901uac5A8egfBuZVHMrDZsQzR96IqNBivCy",
			want:     true,
		}, {
			name:     "sha256",
			password: "$5$1234567890123456$v.2bOKKLlpmUSKn0rxJmgnh.e3wOKivAVNZmNrOsoA3",
			want:     true,
		}, {
			name:     "sha512",
			password: "$6$1234567890123456$d.pgKQFaiD8bRiExg5NesbGR/3u51YvxeYaQXPzx4C6oSYREw8VoReiuYZjx0V9OhGVTZFqhc6emAxT1RC5BV.",
			want:     true,
		}, {
			name:     "scrypt",
			password: "$7$123456789012345", //not actual hash output from scrypt
			want:     false,
		}, {
			name:     "plain",
			password: "password",
			want:     false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := PasswordIsCrypted(test.password); got != test.want {
				t.Errorf("PasswordIsCrypted() =%v, want %v", got, test.want)
			}
		})
	}
}

func TestCryptSHA512(t *testing.T) {
	retPassFirst, err := CryptSHA512("testPass")
	assert.NoError(t, err)
	retPassSecond, _ := CryptSHA512("testPass")
	expectedPassStart := "$6$"
	assert.Equal(t, expectedPassStart, retPassFirst[0:3])
	assert.NotEqual(t, retPassFirst, retPassSecond)
}

func TestGenSalt(t *testing.T) {
	length := 10
	retSaltFirst, err := genSalt(length)
	assert.NoError(t, err)
	retSaltSecond, _ := genSalt(length)
	assert.NotEqual(t, retSaltFirst, retSaltSecond)
}
