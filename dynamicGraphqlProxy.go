package dynamicGraphqlProxy

import (
	"log"
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
	stage := proxy.config.StageConfig.FindCurrentStage(context)
	if _, ok := proxy.config.StageConfig.Stages[stage]; !ok {
		return context.JSON(http.StatusBadRequest, "Stage "+stage+" not existing")
	}
	confHost := strings.Replace(host, proxy.config.StageConfig.Stages[stage], proxy.config.StageConfig.StageKeyWord, 1)

	// find productConfig
	if productConfig, ok := proxy.config.ProductConfigs[confHost]; ok {
		schema, err := productConfig.BuildSchema(context)
		if err != nil {
			context.JSON(http.StatusInternalServerError, "Can't build schema. Please contact Backend-Devs.")
		}
		handler.New(&handler.Config{
			Schema:   schema,
			Pretty:   true,
			GraphiQL: true,
		}).ServeHTTP(context.Response(), context.Request())
		return nil
	}
	log.Println("No schema existing for " + confHost)
	return context.JSON(http.StatusBadGateway, "No schema existing for "+confHost)
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

// Config holds all configs
type Config struct {
	StageConfig    StageConfig
	ProductConfigs map[string]ProductConfig
}

// ProductConfig describes a product
type ProductConfig struct {
	BuildSchema func(context echo.Context) (*graphql.Schema, error)
}

// StageConfig is necessary if you need stages
/*
For example you have following stages:
myProduct-stageA.example.com
myProduct-stageB.example.com

Then is the following config correct:
dynamicGraphqlProxy.StageConfig{
	StageKeyWord: "<stage>",
	Stages:       map[string]string{"A": "-stageA", "B": "-stageB"},
	FindCurrentStage: func(context echo.Context) string {
		return os.Getenv("stage")
	},
}
*/
type StageConfig struct {
	StageKeyWord     string
	Stages           map[string]string
	FindCurrentStage func(context echo.Context) string
}
