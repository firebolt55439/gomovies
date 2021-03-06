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
	airplay "github.com/gongo/go-airplay"
)

// MovieService provides operations on strings.
type MovieService interface {
	Movies(map[string]interface{}, context.Context) (map[string]interface{}, error)
	Count(string) int
}

type movieService struct{}

var client *airplay.Client = nil

func (movieService) Movies(s map[string]interface{}, ctx context.Context) (err_return_value map[string]interface{}, err_return error) {
	defer func() {
        if r := recover(); r != nil {
            err_return = fmt.Errorf("Service was panicking, recovered value: %v (%s)", r, identifyPanic())
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
			_, autoclear_enabled := req_data["autoclear_enabled"]

			/* Execute request */
			payload := map[string]interface{}{
				configuration.DownloadUriOauthParam: uri,
			}
			outp, err := oAuth.Query(configuration.DownloadUriOauth, payload)
			if err != nil {
				return nil, err
			}

			/* If not enough space, clear main folder and try again (only if flag enabled) */
			not_enough_space := false
			if result, ok := outp["result"].(string); ok &&
			(strings.Contains(result, "not_enough_space") || strings.Contains(result, "queue_full")) {
				not_enough_space = true
			}
			if not_enough_space && autoclear_enabled {
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
			if !not_enough_space {
				downloadPool.RegisterOAuthDownloadStart(
					req_data["imdb_id"].(string),
					fmt.Sprintf("%.0f", outp[configuration.CloudItemIdKey].(float64)),
					outp[configuration.CloudHashIdKey].(string),
					outp["title"].(string),
				)
				outp["enqueued"] = false
			} else {
				downloadPool.RegisterOAuthDownloadQueued(
					req_data["imdb_id"].(string),
					payload,
				)
				outp["enqueued"] = true
			}
			outp["not_enough_space"] = not_enough_space
			return outp, err
		case "startBackgroundDownload":
			cloud_id := req_data["id"].(string)
			dl_url := req_data["uri"].(string)
			filename := req_data["filename"].(string)
			err := downloadPool.StartBackgroundDownload(dl_url, cloud_id, filename)
			if err != nil {
				return map[string]interface{}{
					"result": false,
					"err": err.Error(),
				}, nil
			}
			return map[string]interface{}{
				"result": true,
			}, /*err=*/nil
		case "evictLocalItem":
			cloud_id := req_data["id"].(string)
			err := downloadPool.EvictLocalItem(cloud_id)
			if err != nil {
				return map[string]interface{}{
					"result": false,
					"err": err.Error(),
				}, nil
			}
			return map[string]interface{}{
				"result": true,
			}, /*err=*/nil
		case "intelligentRenameItem":
			cloud_id := req_data["id"].(string)
			item_title := req_data["title"].(string)
			new_name, err := downloadPool.IntelligentRenameItem(cloud_id, item_title)
			if err != nil {
				return map[string]interface{}{
					"result": false,
					"err": err.Error(),
				}, nil
			}
			return map[string]interface{}{
				"result": true,
				"new_name": new_name,
			}, /*err=*/nil
		case "getiCloudStreamUrl":
			cloud_id := req_data["id"].(string)
			url, err := downloadPool.GetiCloudStreamUrl(cloud_id)
			if err != nil {
				return map[string]interface{}{
					"result": false,
					"err": err.Error(),
				}, nil
			}
			return map[string]interface{}{
				"result": true,
				"url": url,
			}, /*err=*/nil
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

			/* Refresh and process current download states */
			downloadPool.RefreshDownloadStates(list)
			downloadPool.RefreshDiskDownloads()

			/* Retrieve all downloads from pool */
			download_list := downloadPool.RetrieveDownloads()

			/* Add Airplay playback information, if applicable */
			ret := map[string]interface{}{
				"downloads": download_list,
				"airplay_info": map[string]interface{}{
					"currently_playing": false,
				},
			}
			if client != nil {
				info, err := client.GetPlaybackInfo()
				if err == nil && info.IsReadyToPlay {
					ret["airplay_info"].(map[string]interface{})["currently_playing"] = true
					ret["airplay_info"].(map[string]interface{})["duration"] = info.Duration
					ret["airplay_info"].(map[string]interface{})["position"] = info.Position
				}
			}

			/* Return in desired format. */
			return ret, err
		case "getCollections":
			return map[string]interface{}{
				"collections": downloadPool.GetCollections(),
			}, /*err=*/nil
		case "addToCollection":
			cloud_id := req_data["cloud_id"].(string)
			collection_id := req_data["collection_id"].(string)
			err := downloadPool.AddToCollection(cloud_id, collection_id)
			if err != nil {
				return map[string]interface{}{
					"result": false,
					"err": err.Error(),
				}, nil
			}
			return map[string]interface{}{
				"result": true,
			}, /*err=*/nil
		case "getAssociatedDownloads":
			return map[string]interface{}{
				"downloads": downloadPool.GetAssociatedDownloads(),
			}, /*err=*/nil
		case "associateDownload":
			res := downloadPool.AssociateDownloadWithImdb(
				req_data["cloud_id"].(string),
				req_data["imdb_id"].(string),
			)
			return map[string]interface{}{
				"result": res,
			}, /*err=*/nil
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
		case "updateScrobble":
			progress, ok := req_data["progress"]
			if !ok {
				return nil, errors.New("Parameter `item_type` is required")
			}
			imdb_code, ok := req_data["imdb_code"]
			if !ok {
				return nil, errors.New("Parameter `imdb_code` is required")
			}
			state, ok := req_data["state"]
			if !ok {
				return nil, errors.New("Parameter `state` is required")
			}
			data, err := movieWorker.UpdateScrobbleStatus(imdb_code.(string), progress.(float64), state.(string))
			return data, err
		case "getScrobbles":
			data, err := movieWorker.GetPlaybackScrobbles(lb_ip.(string))
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
		case "startAirplayPlayback":
			url, ok := req_data["url"]
			if !ok {
				return nil, errors.New("Parameter `url` is required")
			}
			progress, ok := req_data["progress"]
			if !ok {
				return nil, errors.New("Parameter `progress` is required")
			}
			if client == nil {
				var err error
				client, err = airplay.FirstClient()
				if err != nil {
					return nil, err
				}
			}
			client.PlayAt(url.(string), progress.(float64))
			return map[string]interface{}{
				"result": true,
			}, nil
		case "stopAirplayPlayback":
			if client == nil {
				return nil, errors.New("No Airplay playback currently occurring")
			}
			client.Stop()
			return map[string]interface{}{
				"result": true,
			}, nil
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
