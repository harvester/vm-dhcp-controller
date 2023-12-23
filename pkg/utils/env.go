package utils

import (
	"os"
	"strconv"
)

func EnvGetBool(key string, defaultValue bool) bool {
	if parsed, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return parsed
	}
	return defaultValue
}
