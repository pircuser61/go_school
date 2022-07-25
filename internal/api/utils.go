package api

const (
	defaultLimit  = 10
	defaultOffset = 0
)

func ptrToInt(v *int) int {
	if v != nil {
		return *v
	}
	return 0
}

func parseLimitOffsetWithDefault(limit, offset *int) (lim, off int) {
	lim, off = defaultLimit, defaultOffset
	if limit != nil {
		lim = *limit
	}
	if offset != nil {
		off = *offset
	}
	return lim, off
}
