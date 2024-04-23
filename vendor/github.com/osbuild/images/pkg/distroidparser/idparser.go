package distroidparser

import (
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/fedora"
	"github.com/osbuild/images/pkg/distro/rhel/rhel10"
	"github.com/osbuild/images/pkg/distro/rhel/rhel7"
	"github.com/osbuild/images/pkg/distro/rhel/rhel8"
	"github.com/osbuild/images/pkg/distro/rhel/rhel9"
)

var DefaultParser = NewDefaultParser()

type ParserFunc func(idStr string) (*distro.ID, error)

// Parser is a list of distro-specific idStr parsers.
type Parser struct {
	parsers []ParserFunc
}

func New(parsers ...ParserFunc) *Parser {
	return &Parser{parsers: parsers}
}

// Parse returns the distro.ID that matches the given distro ID string. If no
// distro.ID matches the given distro ID string, it returns nil. If multiple
// distro.IDs match the given distro ID string, it panics.
// If no distro-specific parser matches the given distro ID string, it falls back
// to the default parser.
//
// The fact that the Parser returns a distro.ID does not mean that the distro is
// actually supported or implemented. This functionality is provided as an easy
// and consistent way to parse distro IDs, while allowing distro-specific parsers.
func (p *Parser) Parse(idStr string) (*distro.ID, error) {
	var match *distro.ID
	for _, f := range p.parsers {
		if d, err := f(idStr); err == nil {
			if match != nil {
				panic("distro ID was matched by multiple parsers")
			}
			match = d
		}
	}

	// Fall back to the default parser
	if match == nil {
		return distro.ParseID(idStr)
	}

	return match, nil
}

// Standardize returns the standardized distro ID string for the given distro ID
// string. If the given distro ID string is not valid, it returns an error.
func (p *Parser) Standardize(idStr string) (string, error) {
	id, err := p.Parse(idStr)
	if err != nil {
		return "", err
	}

	return id.String(), nil
}

func NewDefaultParser() *Parser {
	return New(
		fedora.ParseID,
		rhel7.ParseID,
		rhel8.ParseID,
		rhel9.ParseID,
		rhel10.ParseID,
	)
}
