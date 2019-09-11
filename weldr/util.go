package weldr

import (
	"errors"
	"net/url"
	"strconv"
)

func parseOffsetAndLimit(query url.Values) (uint, uint, error) {
	var offset, limit uint64 = 0, 10
	var err error

	if v := query.Get("offset"); v != "" {
		offset, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid value for 'offset': " + err.Error())
		}
	}

	if v := query.Get("limit"); v != "" {
		limit, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid value for 'limit': " + err.Error())
		}
	}

	return uint(offset), uint(limit), nil
}

func min(a, b uint) uint {
	if a < b {
		return a
	} else {
		return b
	}
}
