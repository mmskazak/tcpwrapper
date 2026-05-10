package isrequest

// IsFirstThreeZero checks if the first three bytes of the message are zero.
// Returns false if the message has fewer than 3 bytes to prevent panic.
func IsFirstThreeZero(message []byte) bool {
	if len(message) < 3 {
		return false
	}
	return message[0] == 0 && message[1] == 0 && message[2] == 0
}
