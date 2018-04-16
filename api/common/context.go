package common

import (
	"github.com/labstack/echo"
	"s7ab-platform-hyperledger/platform/core/api/common"
	"s7ab-platform-hyperledger/platform/core/logger"
)

const (
	defaultChannel = `mychannel`
)

type Context struct {
	common.Context
	SDK *PaymentSDK
}

// NewContext
// Get new context instance
func NewContext(e echo.Context, s *PaymentSDK, l logger.Logger) Context {
	c := Context{}
	c.Context = common.NewContext(e, &s.SDKCore, l)
	c.SDK = s
	return c
}

// InitSDK
// Get SDK instance of presented Organization
func (c *Context) InitSDK(org string) (*PaymentSDK, error) {
	return InitSDK(org, defaultChannel, c.Log)
}
