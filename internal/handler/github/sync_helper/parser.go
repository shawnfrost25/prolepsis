package synchelper

import (
	"errors"
	"strconv"
)

func ParseId(fullDatabaseId *string) (int64, error, string) {
	if fullDatabaseId == nil || *fullDatabaseId == "" {
		return 0, errors.New("Error: Required value 'fullDatabaseId' cannot be empty"), "_id_invalid"
	}
	id, err := strconv.ParseInt(*fullDatabaseId, 10, 64)
	if err != nil {
		return 0, err, "_id_malformed"
	}
	if id <= 0 {
		return 0, errors.New("Error: Value 'fullDatabaseId' cannot be 0 or negative"), "_id_format_invalid"
	}
	return id, nil, ""
}
