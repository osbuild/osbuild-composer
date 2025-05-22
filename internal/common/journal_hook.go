// Inspired by github.com/wercker/journalhook (MIT license)
package common

import (
	"fmt"
	"strings"

	"github.com/coreos/go-systemd/v22/journal"
	logrus "github.com/sirupsen/logrus"
)

type JournalHook struct{}

var (
	severityMap = map[logrus.Level]journal.Priority{
		logrus.DebugLevel: journal.PriDebug,
		logrus.InfoLevel:  journal.PriInfo,
		logrus.WarnLevel:  journal.PriWarning,
		logrus.ErrorLevel: journal.PriErr,
		logrus.FatalLevel: journal.PriCrit,
		logrus.PanicLevel: journal.PriEmerg,
	}
)

func stringifyOp(r rune) rune {
	switch {
	case r >= 'A' && r <= 'Z':
		return r
	case r >= '0' && r <= '9':
		return r
	case r == '_':
		return r
	case r >= 'a' && r <= 'z':
		return r - 32
	default:
		return rune('_')
	}
}

func stringifyKey(key string) string {
	key = strings.Map(stringifyOp, key)
	key = strings.TrimPrefix(key, "_")
	return key
}

// Journal wants strings but logrus takes anything.
func stringifyEntries(data map[string]interface{}) map[string]string {
	entries := make(map[string]string)
	for k, v := range data {

		key := stringifyKey(k)
		entries[key] = fmt.Sprint(v)
	}
	return entries
}

func (hook *JournalHook) Fire(entry *logrus.Entry) error {
	return journal.Send(entry.Message, severityMap[entry.Level], stringifyEntries(entry.Data))
}

func (hook *JournalHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
