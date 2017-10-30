package dynamicGraphqlProxy

import (
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"

	"github.com/labstack/echo"
)

const ProxyParamType = "ProxyParam"

// Proxy hold the config
type Proxy struct {
	*echo.Echo
}

// NewProxy creates a new instance of Proxy
func NewProxy() *Proxy {
	proxy := Proxy{Echo: echo.New()}
	return &proxy
}

// UseProxy is the start-methode
func (proxy *Proxy) UseProxy(config Config) {
	proxy.Any("/:"+ProxyParamType, func(context echo.Context) error {
		host := context.Request().Host
		if config.StageConfig.StageKeyWord != "" {
			stage := config.StageConfig.FindCurrentStage(context)
			if _, ok := config.StageConfig.Stages[stage]; !ok {
				return context.JSON(http.StatusBadRequest, "Stage "+stage+" not existing")
			}
			host = strings.Replace(host, config.StageConfig.Stages[stage], config.StageConfig.StageKeyWord, 1)
		}
		// find productConfig
		if productConfig, ok := config.ProductConfigs[host]; ok {
			route := context.Param(ProxyParamType)
			if delination, okD := productConfig.Delinations[route]; okD {
				var handler echo.HandlerFunc
				handler = func(c echo.Context) error {
					// serve product
					if err := wrapSchema(delination)(context); err != nil {
						return context.JSON(http.StatusInternalServerError, err)
					}
					return nil
				}
				// Chain middleware
				for i := len(delination.MiddlewareModules) - 1; i >= 0; i-- {
					handler = delination.MiddlewareModules[i](handler)
				}
				return handler(context)
			}
			return context.JSON(http.StatusBadGateway, "No route existing for "+route)
		}
		return context.JSON(http.StatusBadGateway, "No schema existing for "+host)
	})
}

// UseProxyWithLocalhost handle a localhost call for your tests
func (proxy *Proxy) UseProxyWithLocalhost(config Config, productHost string) {
	proxy.Any("/local/:"+ProxyParamType, func(context echo.Context) error {
		if productConfig, ok := config.ProductConfigs[productHost]; ok {
			route := context.Param(ProxyParamType)
			if delination, okD := productConfig.Delinations[route]; okD {
				// serve product
				if err := wrapSchema(delination)(context); err != nil {
					return context.JSON(http.StatusInternalServerError, err)
				}
				return nil
			}
			return context.JSON(http.StatusBadGateway, "No route existing for "+route)
		}
		return context.JSON(http.StatusBadGateway, "No schema existing for "+productHost)
	})
}

// Config holds all configs
type Config struct {
	StageConfig    StageConfig
	ProductConfigs map[string]ProductConfig
}

// ProductConfig describes a product
type ProductConfig struct {
	// Delinations is route -> Delination
	Delinations map[string]Delineation
}

// StageConfig is necessary if you need stages
type StageConfig struct {
	StageKeyWord     string
	Stages           map[string]string
	FindCurrentStage func(context echo.Context) string
}

// DelineationType is the type of the delination. Use the const-enums
type DelineationType string

const (
	// Graphql handles Schema as Graphql
	Graphql DelineationType = "GRAPHQL"

	// Rest handles Schema as Rest
	Rest DelineationType = "REST"
)

// Delineation describes a schema
type Delineation struct {
	DelineationType
	MiddlewareModules []echo.MiddlewareFunc
	Schema            *graphql.Schema
}
