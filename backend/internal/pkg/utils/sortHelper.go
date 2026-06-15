package utils

import "time"


func TimeCompare(a, b time.Time) bool {
	if a.IsZero() && b.IsZero() { return false }
    if a.IsZero() { return false } // put zero at end
    if b.IsZero() { return true }
    return a.Before(b)
}