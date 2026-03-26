package handlers

import (
	wasmevents "github.com/Cloud-RAMP/wasm-sandbox/pkg/wasm-events"
)

func SendMessageHandler(event *wasmevents.WASMEventInfo) (string, error) {

	// send test event
	// err = comm.SendEvent(&comm.CommEvent{
	// 	Instance:  instanceId,
	// 	DstRoom:   room,
	// 	DstConn:   connId.String(),
	// 	SrcRoom:   room,
	// 	SrcConn:   connId.String(),
	// 	Payload:   "testing message",
	// 	EventType: comm.SEND_MESSAGE,
	// })
	// if err != nil {
	// 	fmt.Println("error sending msg", err)
	// }

	return "dummy", nil
}
