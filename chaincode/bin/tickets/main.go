package main

import (
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"s7ab-platform-hyperledger/platform/core/logger"
	"s7ab-platform-hyperledger/platform/s7ticket/chaincode"
)

func main() {
	l := logger.NewZapLogger(nil)
	cc := chaincode.NewTicket(l)

	if err := shim.Start(cc); err != nil {
		l.Warn(`chaincode`, logger.KV(`error`, err))
	}
}
