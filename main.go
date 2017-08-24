package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"fmt"
	"encoding/json"

	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-kit/kit/log"
	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	httptransport "github.com/go-kit/kit/transport/http"
)

func main() {
	/* Parse command-line args */
	var (
		listen = flag.String("listen", ":8080", "HTTP listen address")
		proxy  = flag.String("proxy", "", "Optional comma-separated list of URLs to proxy movies requests")
	)
	flag.Parse()
	
	/* Parse configuration file */
	__file, _ := os.Open("config.json")
	__decoder := json.NewDecoder(__file)
	__err := __decoder.Decode(&configuration)
	if __err != nil {
		fmt.Println("Error parsing configuration:", __err)
		panic("Could not parse config")
	}/* else {
		fmt.Println(configuration)
	}*/
	
	/* Initialize OAuth */
	oAuth = OAuth{
		username: configuration.Username,
		password: configuration.Password,
		grant_type: configuration.GrantType,
		client_id: configuration.ClientId,
		access_token_url: configuration.AccessTokenUrl,
		refresh_token_url: configuration.RefreshTokenUrl,
		api_url: configuration.ApiUrl,
	}
	
	/* Initialize microservices */
	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "listen", *listen, "caller", log.DefaultCaller)

	fieldKeys := []string{"method", "error"}
	requestCount := kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_count",
		Help:      "Number of requests received.",
	}, fieldKeys)
	requestLatency := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "request_latency_microseconds",
		Help:      "Total duration of requests in microseconds.",
	}, fieldKeys)
	countResult := kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
		Namespace: "my_group",
		Subsystem: "string_service",
		Name:      "count_result",
		Help:      "The result of each count method.",
	}, []string{})

	var svc MovieService
	svc = movieService{}
	svc = proxyingMiddleware(context.Background(), *proxy, logger)(svc)
	svc = loggingMiddleware(logger)(svc)
	svc = instrumentingMiddleware(requestCount, requestLatency, countResult)(svc)

	moviesHandler := httptransport.NewServer(
		makeMoviesEndpoint(svc),
		decodeMoviesRequest,
		encodeResponse,
		httptransport.ServerBefore(httptransport.PopulateRequestContext),
	)
	countHandler := httptransport.NewServer(
		makeCountEndpoint(svc),
		decodeCountRequest,
		encodeResponse,
	)

	http.Handle("/movies", moviesHandler)
	http.Handle("/count", countHandler)
	http.Handle("/metrics", promhttp.Handler())
	logger.Log("msg", "HTTP", "addr", *listen)
	logger.Log("err", http.ListenAndServe(*listen, nil))
}