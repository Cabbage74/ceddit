package controller

type Code int64

const (
	CodeSuccess Code = 1000 + iota
	CodeInvalidParam
	CodeUserExist
	CodeUserNotExist
	CodeInvalidPassword
	CodeServerBusy

	CodeFail
)

var codeMsgMap = map[Code]string{
	CodeSuccess:         "success",
	CodeInvalidParam:    "invalid param",
	CodeUserExist:       "user exist",
	CodeUserNotExist:    "user not exist",
	CodeInvalidPassword: "invalid password",
	CodeServerBusy:      "server busy",
	CodeFail:            "just fail",
}

func (c Code) Msg() string {
	msg, ok := codeMsgMap[c]
	if !ok {
		msg = codeMsgMap[CodeFail]
	}
	return msg
}
