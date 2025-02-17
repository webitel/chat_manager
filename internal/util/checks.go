package util

import "strconv"

func IsInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
