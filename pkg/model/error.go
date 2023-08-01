package model

const (
	ErrCodeThingOffline = 601
)

type HttpErr struct {
	msg      string
	HttpCode int
	Code     int
}

func (h HttpErr) Error() string {
	return h.msg
}

func MkHttpErr(msg string, httpCode, code int) HttpErr {
	return HttpErr{msg, httpCode, code}
}

var (
	ErrAuthentication         = MkHttpErr("authentication failed", 401, 401)
	ErrAuthorization          = MkHttpErr("authorization failed", 403, 403)
	ErrInvalidParams          = MkHttpErr("invalid parameters", 400, 400)
	ErrNotFound               = MkHttpErr("entity not found", 404, 404)
	ErrDuplicated             = MkHttpErr("entity already exists", 400, 400)
	ErrVersionConflict        = MkHttpErr("version conflict", 409, 409)
	ErrInvalidStateTransition = MkHttpErr("an invalid state transition was attempted", 409, 409)
	ErrPayloadTooLarge        = MkHttpErr("payload too large", 413, 413)

	ErrInternal = MkHttpErr("server internal error", 500, 500)

	ErrShadowFormat = MkHttpErr("invalid shadow format", 400, 400)

	ErrDirectMethodThingOffline = MkHttpErr("thing is offline", 200, ErrCodeThingOffline)
	ErrDirectMethodTimeout      = MkHttpErr("method timeout", 200, 504)
)
