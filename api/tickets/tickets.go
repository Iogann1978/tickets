package tickets

import (
	"github.com/labstack/echo"
	"s7ab-platform-hyperledger/platform/s7ticket/api/tickets/handlers"
)

const (
	DefaultUrlPath = `/tickets`
)

func NewModule(e *echo.Echo, urlPath string, m ...echo.MiddlewareFunc) {
	if urlPath == `` {
		urlPath = DefaultUrlPath
	}
	g := e.Group(urlPath, m...)
	setRouter(g)
}

func setRouter(g *echo.Group) {

	g.GET(`/merchant`, handlers.GetMerchantHandler)

	g.GET("/agent", handlers.AgentHandler)
	g.GET("/agent/list", handlers.AgentListHandler)

	g.POST("/agent/add", handlers.AddAgentHandler)
	// Отправка запроса на платеж в синхронном режиме
	g.POST(`/sync/payment`, handlers.CreateSyncPaymentHandler)
	// Выписка или аннулирование билета
	g.POST(`/sync/payment/:id`, handlers.UpdatePaymentHandler)
	// Получение истории state билета
	g.GET(`/sync/history/:id`, handlers.GetPaymentHistory)
	// Получение информации о платеже
	g.GET(`/payment/:id`, handlers.GetPaymentHandler)
	// Получение списка платежек
	g.GET(`/payment`, handlers.ListPaymentHandler)
	// Системный метод разруливания кто мерчант, а кто агент
	g.POST(`/system/init`, handlers.MerchantInitHandler)
}
