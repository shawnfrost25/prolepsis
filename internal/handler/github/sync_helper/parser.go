package synchelper

import (
	"prolepsis/internal/failure"
	"strconv"
)

func ParseId(fullDatabaseId *string) (int64, error, string) {
	if fullDatabaseId == nil || *fullDatabaseId == "" {
		return 0, failure.EmptyValue("fullDatabaseId"), ""
	}
	id, err := strconv.ParseInt(*fullDatabaseId, 10, 64)
	if err != nil {
		return 0, err, "_id_malformed"
	}
	if id <= 0 {
		return 0, failure.Invalid("fullDatabaseId", "Cannot be negative or 0"), "_id_invalid"
	}
	return id, nil, ""
}
