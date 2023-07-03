package main

const (
	API_PATH_GET_AUTH_QR           string = "/api/get-auth-qr"
	API_PATH_VERIFICATION_CALLBACK string = "/api/verification-callback"
)

const (
	WEBSOCKET_PATH_GET_SESSION_ID string = "/ws"
)

type FunctionName string

const (
	GET_AUTH_QR         FunctionName = "getAuthQr"
	HANDLE_VERIFICATION FunctionName = "handleVerification"
)

type RequestStatus string

const (
	IN_PROGRESS RequestStatus = "IN_PROGRESS"
	ERROR       RequestStatus = "ERROR"
	DONE        RequestStatus = "DONE"
)
