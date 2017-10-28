package dynamicGraphqlProxy_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/TobiEiss/dynamicGraphqlProxy"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"github.com/labstack/echo"
)

func buildTestSchema(context echo.Context) (*graphql.Schema, error) {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "testQuery",
			Fields: graphql.Fields{
				"testField": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	return &schema, err
}

func TestProductConfig(t *testing.T) {
	config := dynamicGraphqlProxy.Config{
		ProductConfigs: map[string]dynamicGraphqlProxy.ProductConfig{
			"myProduct<stage>.example.com": dynamicGraphqlProxy.ProductConfig{
				BuildSchema: buildTestSchema,
			},
		},
		StageConfig: dynamicGraphqlProxy.StageConfig{
			StageKeyWord: "<stage>",
			Stages:       map[string]string{"A": "-stageA", "B": "-stageB", "": ""},
			FindCurrentStage: func(context echo.Context) string {
				return os.Getenv("stage")
			},
		},
	}

	proxy := dynamicGraphqlProxy.NewProxy(config)

	testData := []struct {
		Host         string
		ResponseCode int
		Stage        string
	}{{
		Host:         "myProduct-stageA.example.com",
		Stage:        "A",
		ResponseCode: http.StatusOK,
	}, {
		Host:         "myProduct-stageA.example.com",
		Stage:        "C",
		ResponseCode: http.StatusBadRequest,
	}, {
		Host:         "localhost:8080/localhost",
		ResponseCode: http.StatusOK,
	}, {
		Host:         "myProduct-stageA.example.com",
		ResponseCode: http.StatusBadGateway,
	}}

	for testIndex, test := range testData {
		os.Setenv("stage", test.Stage)

		// build request
		router := echo.New()
		request := httptest.NewRequest(echo.GET, "http://"+test.Host+"/graphql", strings.NewReader(testutil.IntrospectionQuery))
		rec := httptest.NewRecorder()
		router.Any("/graphql", proxy.Handle)
		router.Any("/localhost/graphql", proxy.HandleLocalhost("myProduct<stage>.example.com"))

		// TEST
		router.ServeHTTP(rec, request)
		if rec.Result().StatusCode != test.ResponseCode {
			t.Errorf("[%d] current:%d expected:%d; body:%s", testIndex, rec.Result().StatusCode, test.ResponseCode, rec.Body.String())
		}
	}
}

func TestModules(t *testing.T) {
	type CContext struct {
		echo.Context
		Collector *[]string
	}
	collector := &[]string{}

	config := dynamicGraphqlProxy.Config{
		ProductConfigs: map[string]dynamicGraphqlProxy.ProductConfig{
			"myProduct.example.com": dynamicGraphqlProxy.ProductConfig{
				BuildSchema: buildTestSchema,
				MiddlewareModules: []echo.MiddlewareFunc{
					func(next echo.HandlerFunc) echo.HandlerFunc {
						return func(c echo.Context) error {
							ccontext := c.(*CContext)
							(*ccontext.Collector) = append((*ccontext.Collector), "A")
							return next(c)
						}
					},
					func(next echo.HandlerFunc) echo.HandlerFunc {
						return func(c echo.Context) error {
							ccontext := c.(*CContext)
							(*ccontext.Collector) = append((*ccontext.Collector), "B")
							return next(c)
						}
					},
					func(next echo.HandlerFunc) echo.HandlerFunc {
						return func(c echo.Context) error {
							ccontext := c.(*CContext)
							(*ccontext.Collector) = append((*ccontext.Collector), "C")
							return next(c)
						}
					},
				},
			},
		},
	}
	proxy := dynamicGraphqlProxy.NewProxy(config)

	// build request
	router := echo.New()
	router.Use(func(h echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return h(&CContext{Context: c, Collector: collector})
		}
	})
	request := httptest.NewRequest(echo.GET, "http://myProduct.example.com/graphql", strings.NewReader(testutil.IntrospectionQuery))
	rec := httptest.NewRecorder()
	router.Any("/graphql", proxy.Handle)

	// TEST
	router.ServeHTTP(rec, request)
	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("current:%d expected:%d; body:%s", rec.Result().StatusCode, http.StatusOK, rec.Body.String())
	}

	if len(*collector) < 3 {
		t.Fail()
	}
}
