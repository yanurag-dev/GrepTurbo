package trigram

// T represents a trigram as a packed 3-byte uint32.
// Bytes are packed as: byte0<<16 | byte1<<8 | byte2
type T uint32

// FromBytes packs three bytes into a trigram.
func FromBytes(a, b, c byte) T {
	return T(uint32(a)<<16 | uint32(b)<<8 | uint32(c))
}

// Bytes unpacks a trigram into its three bytes.
func (t T) Bytes() (byte, byte, byte) {
	return byte(t >> 16), byte(t >> 8), byte(t)
}

// String returns the trigram as a 3-character string.
func (t T) String() string {
	a, b, c := t.Bytes()
	return string([]byte{a, b, c})
}

// Extract returns all overlapping trigrams from s, deduplicated.
func Extract(s string) []T {
	if len(s) < 3 {
		return nil
	}
	seen := make(map[T]struct{})
	out := make([]T, 0, len(s)-2)
	for i := 0; i <= len(s)-3; i++ {
		t := FromBytes(s[i], s[i+1], s[i+2])
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}

// ExtractWithDuplicates returns all overlapping trigrams preserving order,
// including duplicates. Useful for building posting lists.
func ExtractWithDuplicates(s string) []T {
	if len(s) < 3 {
		return nil
	}
	out := make([]T, 0, len(s)-2)
	for i := 0; i <= len(s)-3; i++ {
		out = append(out, FromBytes(s[i], s[i+1], s[i+2]))
	}
	return out
}
