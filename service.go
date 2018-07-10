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
	"strings"
)

// MovieService provides operations on strings.
type MovieService interface {
	Movies(map[string]interface{}, context.Context) (map[string]interface{}, error)
	Count(string) int
}

type movieService struct{}

func (movieService) Movies(s map[string]interface{}, ctx context.Context) (err_return_value map[string]interface{}, err_return error) {
	defer func() {
        if r := recover(); r != nil {
            err_return = errors.New(fmt.Sprintf("Service was panicking, recovered value: %v (%s)", r, identifyPanic()))
        }
    }()
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
			data, err := movieWorker.ResolveImdb(imdb_id)
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
		case "fetchUri":
			uri, ok := req_data["uri"]
			if !ok {
				return nil, errors.New("Parameter `uri` is required")
			}
			
			/* Execute request */
			payload := map[string]interface{}{
				configuration.DownloadUriOauthParam: uri,
			}
			outp, err := oAuth.Query(configuration.DownloadUriOauth, payload)
			if err != nil {
				return nil, err
			}
			
			/* If not enough space, clear main folder and try again */
			if result, ok := outp["result"].(string); ok &&
			(strings.Contains(result, "not_enough_space") || strings.Contains(result, "queue_full")) {
				/* Retrieve main folder */
				res, err := oAuth.ApiCall("folder", "GET", map[string]interface{}{})
				if err != nil {
					return nil, err
				}
				
				/* Get all ID's from main folder. */
				var list []interface{}
				list_tmp, ok := res[configuration.OauthDownloadingPath].([]interface{})
				if !ok {
					return nil, errors.New("Could not retrieve ID's from downloading path")
				}
				list = append(list, list_tmp...)
				list_tmp, ok = res["folders"].([]interface{})
				if !ok {
					return nil, errors.New("Could not retrieve ID's from folders path")
				}
				list = append(list, list_tmp...)
				
				/* Clear out main folder */
				fmt.Println("folders:", list)
				for _, item := range list {
					conv_item, ok := item.(map[string]interface{})
					if !ok {
						return nil, errors.New("Could not convert folder item")
					}
					current_id, ok := conv_item["id"].(float64)
					
					delete_type := "folder"
					if _, ok = conv_item["progress_url"].(string); ok {
						delete_type = strings.TrimSuffix(configuration.OauthDownloadingPath, "s")
					}
					
					_, err := oAuth.Query("delete", map[string]interface{}{
						"delete_arr": "[{\"type\": \"" + delete_type + "\", \"id\": \"" + fmt.Sprintf("%.0f", current_id) + "\"}]",
					})
					if err != nil {
						return nil, err
					}
				}
				
				/* Retry request */
				time.Sleep(1 * time.Second)
				outp, err = oAuth.Query(configuration.DownloadUriOauth, payload)
				if err != nil {
					return nil, err
				}
				fmt.Println("retried:", outp)
			}
			return outp, err
		case "getDownloads":
			/* Retrieve main folder */
			res, err := oAuth.ApiCall("folder", "GET", map[string]interface{}{})
			if err != nil {
				return nil, err
			}
			
			/* Get all folders in main folder. */
			var list []interface{}
			list_tmp, ok := res[configuration.OauthDownloadingPath].([]interface{})
			if !ok {
				return nil, errors.New("Could not retrieve ID's from downloading path")
			}
			list = append(list, list_tmp...)
			list_tmp, ok = res["folders"].([]interface{})
			if !ok {
				return nil, errors.New("Could not retrieve ID's from folders path")
			}
			list = append(list, list_tmp...)
			
			/* Return in desired format. */
			return map[string]interface{}{
				"downloads": list,
			}, err
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
		case "searchForItem":
			// Can take {"id": <imdb_id>}
			// Can take {"keyword": <keyword>}
			data, err := movieWorker.SearchForItem(req_data, lb_ip.(string))
			if err == nil {
				outp := map[string]interface{}{
					"results": data,
				}
				return outp, err
			}
			return nil, err
		case "getWatchlist":
			data, err := movieWorker.GetWatchlist(lb_ip.(string))
			if err == nil {
				outp := map[string]interface{}{
					"watchlist": data,
				}
				return outp, err
			}
			return nil, err
		case "addHistory":
			// Takes {"item_type": <...>, "item_id": <...>}
			item_type, ok := req_data["item_type"]
			if !ok {
				return nil, errors.New("Parameter `item_type` is required")
			}
			item_id, ok := req_data["item_id"]
			if !ok {
				return nil, errors.New("Parameter `item_id` is required")
			}
			data, err := movieWorker.AddWatchHistory(item_type.(string), item_id.(string))
			return data, err
		case "addToWatchlist":
			// Takes {"item_type": <...>, "item_id": <...>}
			item_type, ok := req_data["item_type"]
			if !ok {
				return nil, errors.New("Parameter `item_type` is required")
			}
			item_id, ok := req_data["item_id"]
			if !ok {
				return nil, errors.New("Parameter `item_id` is required")
			}
			data, err := movieWorker.AddToWatchlist(item_type.(string), item_id.(string))
			return data, err
		case "getHistory":
			data, err := movieWorker.GetWatchHistory(lb_ip.(string))
			if err == nil {
				outp := map[string]interface{}{
					"watched": data,
				}
				return outp, err
			}
			return nil, err
		case "itemLookup":
			id, ok := req_data["id"]
			if !ok {
				return nil, errors.New("Parameter `id` is required")
			}
			outp, err := movieWorker.GetItem(id.(string), lb_ip.(string))
			return outp, err
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
		if err := pusher.Push("/static/app.js", nil); err != nil {
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
