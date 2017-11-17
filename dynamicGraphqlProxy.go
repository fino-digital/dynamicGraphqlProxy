package dynamicGraphqlProxy

import (
	"net/http"
	"regexp"

	"github.com/fino-digital/schemaToRest"
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
	proxy.Renderer = schemaToRest.GetTemplateRenderer()
	return &proxy
}

// UseProxy is the start-methode
func (proxy *Proxy) UseProxy(config Config) {
	proxy.Any("/:"+ProxyParamType+"/*", func(context echo.Context) error {
		host := context.Request().Host
		// find productConfig
		for hostRegex, productConfig := range config.ProductConfigs {
			if regexp.MustCompile(hostRegex).MatchString(host) {
				route := context.Param(ProxyParamType)
				if delination, okD := productConfig.Delinations[route]; okD {
					var handler echo.HandlerFunc
					handler = func(c echo.Context) error {
						// serve product
						if err := wrapSchema(delination, route)(context); err != nil {
							return context.JSON(http.StatusInternalServerError, err)
						}
						return nil
					}
					// Chain config-middlewares
					for i := len(config.MiddlewareModules) - 1; i >= 0; i-- {
						handler = config.MiddlewareModules[i](handler)
					}
					// Chain product middleware
					for i := len(delination.MiddlewareModules) - 1; i >= 0; i-- {
						handler = delination.MiddlewareModules[i](handler)
					}
					return handler(context)
				}
				return context.JSON(http.StatusBadGateway, "No route existing for "+route)
			}
		}
		return context.JSON(http.StatusBadGateway, "No schema existing for "+host)
	})
}

// UseProxyWithLocalhost handle a localhost call for your tests
func (proxy *Proxy) UseProxyWithLocalhost(config Config, productHost string) {
	proxy.Any("/local/:"+ProxyParamType+"/*", func(context echo.Context) error {
		for hostRegex, productConfig := range config.ProductConfigs {
			if regexp.MustCompile(hostRegex).MatchString(productHost) {
				route := context.Param(ProxyParamType)
				if delination, okD := productConfig.Delinations[route]; okD {
					// serve product
					if err := wrapSchema(delination, route)(context); err != nil {
						return context.JSON(http.StatusInternalServerError, err)
					}
					return nil
				}
				return context.JSON(http.StatusBadGateway, "No route existing for "+route)
			}
		}
		return context.JSON(http.StatusBadGateway, "No schema existing for "+productHost)
	})
}

// Config holds all configs
type Config struct {
	ProductConfigs    map[string]ProductConfig
	MiddlewareModules []echo.MiddlewareFunc
}

// ProductConfig describes a product
type ProductConfig struct {
	// Delinations is route -> Delination
	Delinations map[string]Delineation
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
