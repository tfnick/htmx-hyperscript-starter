package fwcontext

import "github.com/labstack/echo/v4"

const OpenAPIConsumerContextKey = "open_api_consumer"

type OpenAPIConsumerContext struct {
	KeyID       string
	PartnerID   string
	AccountID   string
	Scopes      []string
	Environment string
}

func SetOpenAPIConsumer(c echo.Context, consumer *OpenAPIConsumerContext) {
	c.Set(OpenAPIConsumerContextKey, consumer)
}

func GetOpenAPIConsumer(c echo.Context) *OpenAPIConsumerContext {
	consumer, ok := c.Get(OpenAPIConsumerContextKey).(*OpenAPIConsumerContext)
	if !ok {
		return nil
	}
	return consumer
}
