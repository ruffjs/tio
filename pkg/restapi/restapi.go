package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Resp[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type H map[string]interface{}

func RespOK[T any](data T) Resp[T] {
	return Resp[T]{Code: 200, Message: "OK", Data: data}
}

func SendResp[T any](w http.ResponseWriter, httpStatus int, res Resp[T]) {
	body, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(500)
		_, _ = fmt.Fprint(w, "internal error: "+err.Error())
		return
	}
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(httpStatus)
	_, _ = w.Write(body)
}
