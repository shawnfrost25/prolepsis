package synchelper

import (
	"prolepsis/internal/failure"
	"strconv"
)

func ParseId(fullDatabaseId *string) (int64, string, error) {
	if fullDatabaseId == nil || *fullDatabaseId == "" {
		return 0, "_id_invalid", failure.EmptyValue("fullDatabaseId")
	}
	id, err := strconv.ParseInt(*fullDatabaseId, 10, 64)
	if err != nil {
		return 0, "_id_malformed", err
	}
	if id <= 0 {
		return 0, "_id_format_invalid", failure.Invalid("fullDatabaseId", "cannot be 0 or negative")
	}
	return id, "", nil
}
