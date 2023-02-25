package fmtbytecount

func ByteCountIEC(b uint) (float64, string) {
	const unit = 1024
	if b < unit {
		return float64(b), "B"
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return float64(b) / float64(div), string("KMGTPE"[exp])
}
