package util

import "time"

func TimeOperationMicroseconds(op func()) int64 {
	start := time.Now()
	op()
	return time.Since(start).Microseconds()
}
