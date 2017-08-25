package main

import (
	"errors"
	"context"
	"io/ioutil"
	"net/http"
	"fmt"
	"time"
	"strconv"
	httptransport "github.com/go-kit/kit/transport/http"
	//"strings"
)

// MovieService provides operations on strings.
type MovieService interface {
	Movies(map[string]interface{}, context.Context) (map[string]interface{}, error)
	Count(string) int
}

type movieService struct{}

func (movieService) Movies(s map[string]interface{}, ctx context.Context) (map[string]interface{}, error) {
	//fmt.Println(s)
	//fmt.Println("Host", ctx.Value(httptransport.ContextKeyRequestHost))
	if len(s) == 0 {
		return nil, ErrEmpty
	}
	movieWorker := movieData{}
	
	// Handle request by type
	req_type := s["type"]
	req_data, ok := s["data"].(map[string]interface{})
	if !ok {
		return nil, errors.New("Cannot handle request data")
	}
	lb_ip, ok := s["__lb_ip__"]
	if !ok {
		// Use this instance as the load balancer if none is specified
		lb_ip = ctx.Value(httptransport.ContextKeyRequestHost).(string)
	}
	switch req_type {
		case "imdbIdLookup":
			// Takes {"id": "..."}
			imdb_id, ok := req_data["id"].(string)
			if !ok {
				return nil, errors.New("Invalid IMDB id")
			}
			data, err := movieWorker.ScrapeImdb(imdb_id)
			return data, err
		case "resolveParallel":
			// Takes {"ids": [...]}
			ids_interface, ok := req_data["ids"].([]interface{})
			if !ok {
				return nil, errors.New("Invalid IMDB id list")
			}
			var ids []string
			for _, id := range ids_interface {
				ids = append(ids, id.(string))
			}
			data, err := movieWorker.ResolveParallel(ids, lb_ip.(string))
			if err == nil {
				outp := map[string]interface{}{
					"resolved": data,
				}
				return outp, err
			}
			return nil, err
		case "oauthTest":
			outp, err := oAuth.GetAccessToken(configuration.Username, configuration.Password)
			if outp != nil {
				outp["is_valid"] = oAuth.TestToken()
				outp["test_output"], outp["test_output_err"] = oAuth.ApiCall("folder", "GET", map[string]interface{}{})
			}
			return outp, err
		case "oauthQuery":
			function, ok := req_data["function"]
			if !ok {
				return nil, errors.New("Parameter `function` is required")
			}
			data, ok := req_data["data"]
			if !ok {
				return nil, errors.New("Parameter `data` is required")
			}
			outp, err := oAuth.Query(function.(string), data.(map[string]interface{}))
			return outp, err
		case "oauthApiCall":
			path, ok := req_data["path"]
			if !ok {
				return nil, errors.New("Parameter `path` is required")
			}
			method, ok := req_data["method"]
			if !ok {
				return nil, errors.New("Parameter `method` is required")
			}
			outp, err := oAuth.ApiCall(path.(string), method.(string), /*data=*/nil)
			return outp, err
		case "oauthDownloadUri":
			uri, ok := req_data["uri"]
			if !ok {
				return nil, errors.New("Parameter `uri` is required")
			}
			payload := make(map[string]interface{})
			payload[configuration.DownloadUriOauthParam] = uri
			outp, err := oAuth.Query(configuration.DownloadUriOauth, payload)
			if err != nil {
				return nil, err
			}
			return outp, err
		case "getRecommendedMovies":
			extension, ok := req_data["extended"].(string)
			if !ok {
				extension = "-1"
			}
			ext_int, _ := strconv.Atoi(extension)
			data, err := movieWorker.GetRecommendedMovies(ext_int, lb_ip.(string))
			if err == nil {
				outp := map[string]interface{}{
					"recommendations": data,
				}
				return outp, err
			}
			return nil, err
		default:
			return nil, errors.New("Invalid request type")
	}
	panic("should never get here")
}

func (movieService) Count(s string) int {
	return len(s)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	defer func(begin time.Time) {
		logger.Log(
			"method", "root",
			"took", time.Since(begin),
		)
	}(time.Now())
	
	if pusher, ok := w.(http.Pusher); ok {
		if err := pusher.Push("/static/app.js?v=0.0.1", nil); err != nil {
			logger.Log("Failed to push: %v", err)
		}
		if err := pusher.Push("/static/style.css", nil); err != nil {
			logger.Log("Failed to push: %v", err)
		}
	}
	
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	data, err := ioutil.ReadFile("static/index.html")
	if err != nil {
		panic(err)
	}
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	fmt.Fprint(w, string(data))
}

// ErrEmpty is returned when an input string is empty.
var ErrEmpty = errors.New("empty string")

// ServiceMiddleware is a chainable behavior modifier for MovieService.
type ServiceMiddleware func(MovieService) MovieService