package ecode

type Code int

const (
	OK              Code = 200
	BadRequest      Code = 400
	Unauthorized    Code = 401
	Forbidden       Code = 403
	NotFound        Code = 404
	TooManyRequests Code = 429
	InternalServer  Code = 500
)

type Error struct {
	Code    Code
	Message string
}

func (e Error) Error() string {
	return e.Message
}

func New(code Code, message string) Error {
	return Error{Code: code, Message: message}
}
