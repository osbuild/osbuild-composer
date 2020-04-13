package fsjobqueue

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func uuidList(t *testing.T, strs ...string) []uuid.UUID {
	var err error
	ids := make([]uuid.UUID, len(strs))
	for i, s := range strs {
		ids[i], err = uuid.Parse(s)
		require.NoError(t, err)
	}
	return ids
}

func TestUniqueUUIDList(t *testing.T) {
	l := uniqueUUIDList([]uuid.UUID{})
	require.Empty(t, l)

	s := uuidList(t, "8ad6bbcd-55f9-4cd8-be45-d0370ff079d2", "a0ad7428-b813-4efb-a156-da2b524f4868", "36e5817c-f29d-4043-8d7d-95ffaa77ff88")
	l = uniqueUUIDList(s)
	require.ElementsMatch(t, s, l)

	s = uuidList(t, "8ad6bbcd-55f9-4cd8-be45-d0370ff079d2", "8ad6bbcd-55f9-4cd8-be45-d0370ff079d2")
	l = uniqueUUIDList(s)
	require.ElementsMatch(t, uuidList(t, "8ad6bbcd-55f9-4cd8-be45-d0370ff079d2"), l)

	s = uuidList(t, "8ad6bbcd-55f9-4cd8-be45-d0370ff079d2", "a0ad7428-b813-4efb-a156-da2b524f4868", "8ad6bbcd-55f9-4cd8-be45-d0370ff079d2")
	l = uniqueUUIDList(s)
	require.ElementsMatch(t, uuidList(t, "8ad6bbcd-55f9-4cd8-be45-d0370ff079d2", "a0ad7428-b813-4efb-a156-da2b524f4868"), l)
}
