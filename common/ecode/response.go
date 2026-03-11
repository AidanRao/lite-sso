package ecode

type Response[T any] struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func OKResponse[T any](data T) Response[T] {
	return Response[T]{Code: OK, Message: "success", Data: data}
}

func ErrorResponse(err Error) Response[any] {
	return Response[any]{Code: err.Code, Message: err.Message, Data: nil}
}
