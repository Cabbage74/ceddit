package response

type Code int64

const (
	CodeSuccess Code = 1000 + iota
	CodeInvalidParam
	CodeUserExist
	CodeUserNotExist
	CodeInvalidPassword
	CodeServerBusy
	CodeNeedAuth
	CodeInvalidAuth
	CodeAuthExpired

	CodeNotOwner
	CodeContentNotOK
	CodeDraftNotReady

	CodeFail
)

var codeMsgMap = map[Code]string{
	CodeSuccess:         "success",
	CodeInvalidParam:    "invalid param",
	CodeUserExist:       "user exist",
	CodeUserNotExist:    "user not exist",
	CodeInvalidPassword: "invalid password",
	CodeServerBusy:      "server busy",
	CodeNeedAuth:        "need auth",
	CodeInvalidAuth:     "invalid auth",
	CodeAuthExpired:     "auth expired",
	CodeNotOwner:        "not the owner",
	CodeContentNotOK:    "content verification failed",
	CodeDraftNotReady:   "draft not ready",
	CodeFail:            "just fail",
}

func (c Code) Msg() string {
	msg, ok := codeMsgMap[c]
	if !ok {
		msg = codeMsgMap[CodeFail]
	}
	return msg
}
