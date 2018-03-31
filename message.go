package main

type MessageType int

const (
	RegisterRespType MessageType = iota
	TaskRequestType
	TaskResultType
	LeaveReqType
	LeaveRespType
)

// Slave expects messages like this and then it can parse body field according to the specified id
type Message struct {
	ID   MessageType `json:"id"`
	Body string      `json:"body"`
}

// RegisterResp: When slave connects it should expect this as the first message from master
type RegisterResp struct {
	Code        int    `json:"code"`
	Description string `json:"desc"`
}

// Task: server will ask slave to do some task
type Task struct {
	TransactionID int64  `json:"transaction_id"`
	TargetURL     string `json:"target_url"`
}

// TaskResult: when task is done, slave replies to master
type TaskResult struct {
	TransactionID int64  `json:"transaction_id"`
	Result        string `json:"result"`
	Code          int    `json:"code"`
	Description   string `json:"desc"`
}

// LeaveReq: the slave want to exit
type LeaveReq struct {
}
