package ext

func ByteInArray(b byte, A []byte) bool {
	for _, e := range A {
		if e == b {
			return true
		}
	}
	return false
}
