package dynamicGraphqlProxy

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"github.com/graphql-go/handler"
	"github.com/labstack/echo"
)

// Proxy hold the config
type Proxy struct {
	config Config
}

// NewProxy creates a new instance of Proxy
func NewProxy(config Config) *Proxy {
	proxy := Proxy{config: config}
	proxy.checkIfAllSchemataBuildable()
	return &proxy
}

// Handle is for the echo-router
func (proxy *Proxy) Handle(context echo.Context) error {
	host := context.Request().Host
	if proxy.config.StageConfig.StageKeyWord != "" {
		stage := proxy.config.StageConfig.FindCurrentStage(context)
		if _, ok := proxy.config.StageConfig.Stages[stage]; !ok {
			return context.JSON(http.StatusBadRequest, "Stage "+stage+" not existing")
		}
		host = strings.Replace(host, proxy.config.StageConfig.Stages[stage], proxy.config.StageConfig.StageKeyWord, 1)
	}

	// find productConfig
	if productConfig, ok := proxy.config.ProductConfigs[host]; ok {
		var handler echo.HandlerFunc
		handler = func(c echo.Context) error {
			// serve product
			if err := serveProductConfig(context, productConfig); err != nil {
				return context.JSON(http.StatusInternalServerError, err)
			}
			return nil
		}
		// Chain middleware
		for i := len(productConfig.MiddlewareModules) - 1; i >= 0; i-- {
			handler = productConfig.MiddlewareModules[i](handler)
		}
		return handler(context)

	}
	return context.JSON(http.StatusBadGateway, "No schema existing for "+host)
}

// HandleLocalhost handle a localhost call for your tests
func (proxy *Proxy) HandleLocalhost(host string) func(echo.Context) error {
	return func(context echo.Context) error {
		if productConfig, ok := proxy.config.ProductConfigs[host]; ok {
			if err := serveProductConfig(context, productConfig); err != nil {
				context.JSON(http.StatusInternalServerError, err)
			}
		}
		return context.JSON(http.StatusBadGateway, "No schema existing for "+host)
	}
}

func (proxy *Proxy) checkIfAllSchemataBuildable() {
	testRouter := echo.New()
	context := testRouter.NewContext(
		httptest.NewRequest(echo.GET, "/", strings.NewReader(testutil.IntrospectionQuery)), httptest.NewRecorder())
	for host, productConfig := range proxy.config.ProductConfigs {
		if _, err := productConfig.BuildSchema(context); err != nil {
			panic("[" + host + "]" + " build error: " + err.Error())
		}
	}
}

func serveProductConfig(context echo.Context, productConfig ProductConfig) error {
	schema, err := productConfig.BuildSchema(context)
	if err != nil {
		return errors.New("Can't build schema. Please contact Backend-Devs")
	}
	handler.New(&handler.Config{
		Schema:   schema,
		Pretty:   true,
		GraphiQL: true,
	}).ServeHTTP(context.Response(), context.Request())
	return nil
}

// Config holds all configs
type Config struct {
	StageConfig    StageConfig
	ProductConfigs map[string]ProductConfig
}

// ProductConfig describes a product
type ProductConfig struct {
	MiddlewareModules []echo.MiddlewareFunc
	BuildSchema       func(context echo.Context) (*graphql.Schema, error)
}

// StageConfig is necessary if you need stages
type StageConfig struct {
	StageKeyWord     string
	Stages           map[string]string
	FindCurrentStage func(context echo.Context) string
}
