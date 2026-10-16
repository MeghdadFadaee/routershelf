package smbserver

// compatibleShare applies both handles' SMB share access masks before opening
// or truncating an existing file. This is independent of authentication.
func compatibleShare(a, share, b, other uint32) bool {
	needs := func(access uint32) uint32 {
		var n uint32
		if access&(fileReadData|fileExecute) != 0 {
			n |= 1
		}
		if access&(fileWriteData|fileAppendData) != 0 {
			n |= 2
		}
		if access&accessDelete != 0 {
			n |= 4
		}
		return n
	}
	return needs(a)&^other == 0 && needs(b)&^share == 0
}

// ioAllowed must be called with the server mutex held, including during the
// subsequent I/O. Locks protect data, not just competing LOCK requests.
func ioAllowed(o *open, off, n uint64, write bool) bool {
	for _, l := range o.shared.locks {
		if l.owner != o && (write || l.exclusive) && l.overlaps(off, n) {
			return false
		}
	}
	return true
}
