package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	jujuratelimit "github.com/juju/ratelimit"
	"github.com/sony/gobreaker"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/ratelimit"
	"github.com/go-kit/kit/sd"
	"github.com/go-kit/kit/sd/lb"
	httptransport "github.com/go-kit/kit/transport/http"
)

func proxyingMiddleware(ctx context.Context, instances string, logger log.Logger) ServiceMiddleware {
	// If instances is empty, don't proxy.
	if instances == "" {
		logger.Log("proxy_to", "none")
		return func(next MovieService) MovieService { return next }
	}

	// Set some parameters for our client.
	var (
		qps         = 100000                    // beyond which we will return an error
		maxAttempts = 5                      // per request, before giving up
		maxTime     = 25000 * time.Millisecond // wallclock time, before giving up
	)

	// Otherwise, construct an endpoint for each instance in the list, and add
	// it to a fixed set of endpoints. In a real service, rather than doing this
	// by hand, you'd probably use package sd's support for your service
	// discovery system.
	var (
		instanceList = split(instances)
		endpointer   sd.FixedEndpointer
	)
	logger.Log("proxy_to", fmt.Sprint(instanceList))
	for _, instance := range instanceList {
		var e endpoint.Endpoint
		e = makeMoviesProxy(ctx, instance)
		e = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name: "Proxy Breaker",
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 3 && failureRatio >= 0.6
			},
		}))(e)
		e = ratelimit.NewTokenBucketLimiter(jujuratelimit.NewBucketWithRate(float64(qps), int64(qps)))(e)
		endpointer = append(endpointer, e)
	}

	// Now, build a single, retrying, load-balancing endpoint out of all of
	// those individual endpoints.
	balancer := lb.NewRoundRobin(endpointer)
	retry := lb.Retry(maxAttempts, maxTime, balancer)

	// And finally, return the ServiceMiddleware, implemented by proxymw.
	return func(next MovieService) MovieService {
		return proxymw{ctx, next, retry}
	}
}

// proxymw implements MovieService, forwarding Movies requests to the
// provided endpoint, and serving all other (i.e. Count) requests via the
// next MovieService.
type proxymw struct {
	ctx       context.Context
	next      MovieService     // Serve most requests via this service...
	movies endpoint.Endpoint // ...except Movies, which gets served by this endpoint
}

func (mw proxymw) Count(s string) int {
	return mw.next.Count(s)
}

func GetOutboundIP() net.IP {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    localAddr := conn.LocalAddr().(*net.UDPAddr)

    return localAddr.IP
}

func (mw proxymw) Movies(s map[string]interface{}, ctx context.Context) (map[string]interface{}, error) {
	//fmt.Println("HostProxy", ctx.Value(httptransport.ContextKeyRequestHost))
	if _, ok := s["__lb_ip__"]; !ok && s != nil {
		//s["__host__"] = ctx.Value(httptransport.ContextKeyRequestHost).(string)
		our_ip := GetOutboundIP().String()
		http_host := ctx.Value(httptransport.ContextKeyRequestHost).(string)
		if arr := strings.Split(http_host, ":"); len(arr) > 1 {
			our_ip += ":" + arr[1]
		} else {
			our_ip += ":80"
		}
		s["__lb_ip__"] = our_ip
	}
	response, err := mw.movies(mw.ctx, moviesRequest{S: s})
	if err != nil {
		return nil, err
	}

	resp := response.(moviesResponse)
	if resp.Err != "" {
		return resp.V, errors.New(resp.Err)
	}
	return resp.V, nil
}

func makeMoviesProxy(ctx context.Context, instance string) endpoint.Endpoint {
	if !strings.HasPrefix(instance, "http") {
		instance = "http://" + instance
	}
	u, err := url.Parse(instance)
	if err != nil {
		panic(err)
	}
	if u.Path == "" {
		u.Path = "/movies"
	}
	return httptransport.NewClient(
		"GET",
		u,
		encodeRequest,
		decodeMoviesResponse,
	).Endpoint()
}

func split(s string) []string {
	a := strings.Split(s, ",")
	for i := range a {
		a[i] = strings.TrimSpace(a[i])
	}
	return a
}