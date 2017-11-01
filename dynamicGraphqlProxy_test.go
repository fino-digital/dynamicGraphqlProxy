package dynamicGraphqlProxy_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/fino-digital/dynamicGraphqlProxy"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/testutil"
	"github.com/labstack/echo"
)

func buildTestSchema() (*graphql.Schema, error) {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "testQuery",
			Fields: graphql.Fields{
				"testField": &graphql.Field{
					Type: graphql.String,
					Resolve: func(param graphql.ResolveParams) (interface{}, error) {
						return "Hello", errors.New("Error")
					},
				},
			},
		}),
	})
	return &schema, err
}

func TestHostRegex(t *testing.T) {
	var validHost = regexp.MustCompile(`^(dev\.|test\.)?example\.com$`)

	testData := map[string]bool{
		"dev.example.com":     true,
		"test.example.com":    true,
		"example.com":         true,
		"testing.example.com": false,
	}

	for host, shouldMatch := range testData {
		if validHost.MatchString(host) != shouldMatch {
			t.Fail()
		}
	}
}

func TestHostRegex2(t *testing.T) {
	var validHost = regexp.MustCompile(`^myproduct(-stageA|-stageB)?\.example\.com$`)

	testData := map[string]bool{
		"myproduct-stageA.example.com": true,
		"myproduct-stageB.example.com": true,
		"myproduct.example.com":        true,
		"testing.example.com":          false,
	}

	for host, shouldMatch := range testData {
		if validHost.MatchString(host) != shouldMatch {
			t.Errorf("following failed: %s", host)
		}
	}
}

func TestProductConfig(t *testing.T) {
	schema, err := buildTestSchema()
	if err != nil {
		panic(err)
	}
	config := dynamicGraphqlProxy.Config{
		ProductConfigs: map[string]dynamicGraphqlProxy.ProductConfig{
			`^myproduct(-stageA|-stageB)?\.example\.com$`: dynamicGraphqlProxy.ProductConfig{
				Delinations: map[string]dynamicGraphqlProxy.Delineation{
					"graphql": dynamicGraphqlProxy.Delineation{
						Schema:          schema,
						DelineationType: dynamicGraphqlProxy.Graphql,
					},
				},
			},
		},
	}

	proxy := dynamicGraphqlProxy.NewProxy()
	proxy.UseProxy(config)
	proxy.UseProxyWithLocalhost(config, "myproduct.example.com")

	testData := []struct {
		Host         string
		Route        string
		ResponseCode int
		Stage        string
	}{{
		Host:         "myproduct-stageA.example.com",
		Route:        "/graphql/",
		ResponseCode: http.StatusOK,
	}, {
		Host:         "myproduct-stageB.example.com",
		Route:        "/graphql/",
		ResponseCode: http.StatusOK,
	}, {
		Host:         "localhost:8080/local",
		Route:        "/graphql/",
		ResponseCode: http.StatusOK,
	}, {
		Host:         "myproduct.example.com",
		Route:        "/graphql/",
		ResponseCode: http.StatusOK,
	}, {
		Host:         "myproduct-wrongstage.example.com",
		Route:        "/graphql/",
		ResponseCode: http.StatusBadGateway,
	}}

	for testIndex, test := range testData {
		// build request
		target := "http://" + test.Host + test.Route
		request := httptest.NewRequest(echo.POST, target, strings.NewReader(testutil.IntrospectionQuery))
		rec := httptest.NewRecorder()

		// TEST
		proxy.ServeHTTP(rec, request)
		if rec.Result().StatusCode != test.ResponseCode {
			t.Errorf("[%d] current:%d expected:%d; body:%s", testIndex, rec.Result().StatusCode, test.ResponseCode, rec.Body.String())
		}
	}
}

func TestModules(t *testing.T) {
	collector := []string{}
	schema, _ := buildTestSchema()

	config := dynamicGraphqlProxy.Config{
		ProductConfigs: map[string]dynamicGraphqlProxy.ProductConfig{
			"myProduct.example.com": dynamicGraphqlProxy.ProductConfig{
				Delinations: map[string]dynamicGraphqlProxy.Delineation{
					"graphql": dynamicGraphqlProxy.Delineation{
						Schema:          schema,
						DelineationType: dynamicGraphqlProxy.Graphql,
						MiddlewareModules: []echo.MiddlewareFunc{
							func(next echo.HandlerFunc) echo.HandlerFunc {
								return func(c echo.Context) error {
									collector = append(collector, "A")
									return next(c)
								}
							},
							func(next echo.HandlerFunc) echo.HandlerFunc {
								return func(c echo.Context) error {
									collector = append(collector, "B")
									return next(c)
								}
							},
							func(next echo.HandlerFunc) echo.HandlerFunc {
								return func(c echo.Context) error {
									collector = append(collector, "C")
									return next(c)
								}
							},
						},
					},
				},
			},
		},
	}
	proxy := dynamicGraphqlProxy.NewProxy()
	proxy.UseProxy(config)

	// build request
	request := httptest.NewRequest(echo.POST, "http://myProduct.example.com/graphql/", strings.NewReader(testutil.IntrospectionQuery))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, request)

	// TEST
	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("current:%d expected:%d; body:%s", rec.Result().StatusCode, http.StatusOK, rec.Body.String())
	}

	if len(collector) < 3 {
		t.Fail()
	}
}
