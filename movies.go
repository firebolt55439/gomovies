package main

import (
	"net/http"
	"html"
	"time"
	"io/ioutil"
	"fmt"
	"errors"
	"strings"
	"strconv"
	"encoding/gob"
	"encoding/json"
	"bytes"
	"sort"
	"math"
	"math/rand"
	"runtime"
	"os"

	"github.com/coocood/freecache"
	"github.com/42minutes/go-trakt"
	"github.com/sethgrid/pester"
)

type MovieData interface {
	/* IMDB metadata resolution */
	ScrapeImdb(id string) (map[string]interface{}, error)
	ResolveImdb(id string) (map[string]interface{}, error)
	ResolveParallel(ids []string, load_balancer_addr string) ([]map[string]interface{}, error)

	/* Trakt.tv and taste.io integration */
	GetRecommendedMovies(extension int, load_balancer_addr string) ([]map[string]interface{}, error)
	GetWatchlist(load_balancer_addr string) ([]map[string]interface{}, error)
	AddToWatchlist(item_type string, item_id string) (map[string]interface{}, error)
	AddWatchHistory(item_type string, item_id string) (map[string]interface{}, error)
	GetWatchHistory(load_balancer_addr string) ([]map[string]interface{}, error)

	/* Media sources */
	SearchForItem(opts map[string]interface{}, load_balancer_addr string) ([]map[string]interface{}, error)
	GetItem(id string, load_balancer_addr string) (map[string]interface{}, error)
}

type movieData struct{}

/* Helper functions */
func getAfter(s string, sub string) string {
	ret := strings.Split(s, sub)
	if len(ret) == 1 {
		return ""
	}
	return ret[1]
}

func getBefore(s string, sub string) string {
	ret := strings.Split(s, sub)
	if len(ret) == 1 {
		return ""
	}
	return ret[0]
}

func getBetween(s string, a string, b string) string {
	return getBefore(getAfter(s, a), b)
}

func GetBytes(key interface{}) ([]byte, error) {
    var buf bytes.Buffer
    enc := gob.NewEncoder(&buf)
    err := enc.Encode(key)
    if err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

func identifyPanic() string {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(3, pc[:])
	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	switch {
	case name != "":
		return fmt.Sprintf("%v:%v", name, line)
	case file != "":
		return fmt.Sprintf("%v:%v", file, line)
	}

	return fmt.Sprintf("pc:%x", pc)
}

/* Interface functions */
const (
	IMDB_KEY_ID = "imdbKeyId-"
	ITEM_KEY_ID = "itemKeyId-"
)

var cache *freecache.Cache = freecache.NewCache(20 * 1024 * 1024)
var netClient = pester.New()

func (movieData) ScrapeImdb(id string) (parsed map[string]interface{}, err error) {
	defer func() {
        if r := recover(); r != nil {
            err = errors.New(fmt.Sprintf("ScrapeImdb was panicking, recovered value: %v (%s)", r, identifyPanic()))
        }
    }()
    parsed = make(map[string]interface{})
    err = nil

    // Check cache
    cached, ok := cache.Get([]byte(IMDB_KEY_ID + id))
    if ok == nil && cached != nil {
    	buf := bytes.NewBuffer(cached)
    	dec := gob.NewDecoder(buf)
    	v := dec.Decode(&parsed)
    	if v != nil {
    		panic(v)
    	}
    	return parsed, err
    }

	// Download IMDB url
	parsed["imdb_code"] = id
	imdb_url := fmt.Sprintf("http://www.imdb.com/title/%s/", id)
	var resp (*http.Response)
	for ct := 0;; ct += 1 {
		resp, err = netClient.Get(imdb_url)
		if err != nil {
			if ct > 5 {
				return nil, err
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		break
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	body := string(bytes)
	//fmt.Println(body[0:200])
	resp.Body.Close()

	// Validity check
	if strings.Index(body, "div class=\"title_wrapper\">") == -1 {
		parsed["unreleased"] = true
		return parsed, err
	}

	// Poster image
	poster := getAfter(body, "div class=\"poster\"")
	if len(poster) == 0 {
		parsed["unreleased"] = true
		return parsed, err
	}
	poster = getAfter(getAfter(poster, "img"), "src=\"")
	poster = getBefore(poster, "\"")
	parsed["cover_image"] = poster

	// Year
	year := getAfter(body, "<span id=\"titleYear\">")
	year = getBefore(getAfter(year, ">"), "<")
	parsed["year"], ok = strconv.Atoi(year)

	// Title
	title := getAfter(body, "div class=\"title_wrapper\">")
	title = getBetween(title, ">", "<")
	title = strings.Replace(title, "&nbsp;", "", -1)
	title = strings.TrimSpace(title)
	title = html.UnescapeString(title)
	parsed["title"] = title
	if parsed["year"].(int) > 0 {
		parsed["title"] = fmt.Sprintf("%s (%d)", title, parsed["year"])
	}

	// MPAA Rating
	mpaa_rating := getAfter(body, "meta itemprop=\"contentRating\"")
	mpaa_rating = getBetween(mpaa_rating, "content=\"", "\"")
	parsed["mpaa_rating"] = mpaa_rating

	// IMDb Rating
	imdb_rating := getAfter(body, "span itemprop=\"ratingValue\"")
	unreleased := false
	imdb_rating = getBetween(imdb_rating, ">", "<")
	if len(imdb_rating) == 0 {
		unreleased = true
		imdb_rating = "10.0"
	}
	parsed["unreleased"] = unreleased
	parsed["imdb_rating"], ok = strconv.ParseFloat(imdb_rating, /*bitsize=*/64)

	// IMDB Rating Count
	imdb_rating_count := getAfter(body, "itemprop=\"ratingCount\"")
	imdb_rating_count = getBetween(imdb_rating_count, ">", "<")
	imdb_rating_count = strings.Replace(imdb_rating_count, ",", "", -1)
	if unreleased {
		imdb_rating_count = "1";
	}
	parsed["imdb_rating_count"], _ = strconv.Atoi(imdb_rating_count)

	// Summary
	summary := getAfter(body, "class=\"summary_text\"")
	summary = getBetween(summary, ">", "<")
	//fmt.Println(summary)
	summary = strings.TrimSpace(summary)
	summary = html.UnescapeString(summary)
	parsed["summary"] = summary

	if len(summary) == 0 {
		fmt.Println("ZERO SUM LENGTH (%s)!", id)
	}

	// TV Show Detection
	is_tv_show := strings.Index(mpaa_rating, "TV") != -1
	parsed["is_tv_show"] = is_tv_show

	// Cache result
	parsed_bytes, ok := GetBytes(parsed)
	cache.Set([]byte(IMDB_KEY_ID + id), parsed_bytes, /*24 hours=*/24 * 60 * 60)

	// Return gathered data
	return parsed, err
}

func (movieData) ResolveImdb(id string) (parsed map[string]interface{}, err error) {
	defer func() {
        if r := recover(); r != nil {
            err = errors.New(fmt.Sprintf("ResolveImdb was panicking, recovered value: %v (%s)", r, identifyPanic()))
        }
    }()
    parsed = make(map[string]interface{})
    err = nil

    // Check cache
    cached, ok := cache.Get([]byte(IMDB_KEY_ID + id))
    if ok == nil && cached != nil {
    	buf := bytes.NewBuffer(cached)
    	dec := gob.NewDecoder(buf)
    	v := dec.Decode(&parsed)
    	if v != nil {
    		panic(v)
    	}
    	return parsed, err
    }

	// Download IMDB url
	rand.Seed(time.Now().UnixNano())
	parsed["imdb_code"] = id
	api_key := configuration.OmdbApiKeys[rand.Intn(len(configuration.OmdbApiKeys) - 1)]
	imdb_url := fmt.Sprintf("http://www.omdbapi.com/?i=%s&apikey=%s", id, api_key)
	var resp (*http.Response)
	for ct := 0;; ct += 1 {
		resp, err = netClient.Get(imdb_url)
		if err != nil {
			if ct > 5 {
				return nil, err
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		break
	}
	bytes, _ := ioutil.ReadAll(resp.Body)
	body := string(bytes)
	//fmt.Println(body[0:200])
	resp.Body.Close()

	// fmt.Printf("URL: %s", imdb_url)

	// Parse JSON if possible
	var body_json map[string]interface{}
	if err = json.Unmarshal([]byte(body), &body_json); err != nil {
		parsed["unreleased"] = true
		return parsed, err
	}

	// Parse score and check if unreleased (unreleased if no score available)
	rating, _ := body_json["imdbRating"].(string)
	if rating == "N/A" {
		parsed["unreleased"] = true
		return parsed, err
	}
	rating_int, ok := strconv.ParseFloat(rating, /*bitsize=*/64)
	parsed["imdb_rating"] = rating_int

	// Vote count
	rating_count, _ := body_json["imdbVotes"].(string)
	rating_count = strings.Replace(rating_count, ",", "", -1)
	parsed["imdb_rating_count"], _ = strconv.Atoi(rating_count)

	// Poster image
	parsed["cover_image"], _ = body_json["Poster"].(string)

	// Year
	if body_json["Year"] == nil {
		parsed["unreleased"] = true
		return parsed, err
	}
	year, _ := strconv.Atoi(body_json["Year"].(string))
	parsed["year"] = year

	// Title
	title := body_json["Title"].(string)
	if parsed["year"].(int) > 0 {
		parsed["title"] = fmt.Sprintf("%s (%d)", title, parsed["year"])
	} else {
		parsed["title"] = title
	}

	// MPAA Rating
	parsed["mpaa_rating"] = body_json["Rated"].(string)
	if parsed["mpaa_rating"] == "NOT RATED" || parsed["mpaa_rating"] == "N/A" || parsed["mpaa_rating"] == "UNRATED" {
		parsed["mpaa_rating"] = "NR"
	}

	// Summary
	parsed["summary"] = body_json["Plot"].(string)

	// Runtime
	parsed["runtime"] = body_json["Runtime"].(string)

	// Awards, if/a
	if parsed["awards"] = body_json["Awards"].(string); parsed["awards"] == "N/A" {
		parsed["awards"] = ""
	}

	// Genres
	parsed["genres"] = strings.Split(body_json["Genre"].(string), ", ")

	// Metacritic and/or Rotten Tomatoes, if/a
	parsed["rotten_tomatoes"] = ""
	parsed["metacritic"] = ""
	for _, on_v := range body_json["Ratings"].([]interface{}) {
		on := on_v.(map[string]interface{})
		if on["Source"].(string) == "Rotten Tomatoes" {
			parsed["rotten_tomatoes"], _ = strconv.Atoi(on["Value"].(string)[0:2])
		}
		if on["Source"].(string) == "Metacritic" {
			parsed["metacritic"], _ = strconv.Atoi(on["Value"].(string)[0:2])
		}
	}

	// Cast, if/a
	if parsed["cast"] = body_json["Actors"].(string); parsed["cast"] == "N/A" {
		parsed["cast"] = ""
	}

	// TV Show Detection
	media_type := body_json["Type"].(string)
	parsed["is_tv_show"] = media_type == "series"

	// Cache result
	parsed_bytes, ok := GetBytes(parsed)
	cache.Set([]byte(IMDB_KEY_ID + id), parsed_bytes, /*24 hours=*/24 * 60 * 60)

	// Return gathered data
	return parsed, err
}

func (movieData) ResolveParallel(ids []string, load_balancer_addr string) (ret []map[string]interface{}, err error) {
	defer func() {
        if r := recover(); r != nil {
            err = errors.New(fmt.Sprintf("ResolveParallel was panicking, recovered value: %v (%s)", r, identifyPanic()))
        }
    }()
    err = nil
    parsed := make(chan map[string]interface{}, len(ids))

    // Resolve ID's in parallel via the load-balancer
    for _, imdb_id := range ids {
    	go func(imdb_id string) {
    		to_send := new(bytes.Buffer)
    		json.NewEncoder(to_send).Encode(map[string]interface{}{
    			"q": map[string]interface{}{
    				"type": "imdbIdLookup",
    				"data": map[string]interface{}{
    					"id": imdb_id,
    				},
    			},
    		})
    		posting_url := fmt.Sprintf("http://%s/movies", load_balancer_addr)
    		//fmt.Println(to_send)
    		var res (*http.Response)
    		var ok error
    		ct := 0
    		for ;; {
    			ct += 1
				res, ok = netClient.Post(
					posting_url,
					"application/json; charset=utf-8",
					to_send,
				)
				if ok != nil {
					fmt.Println("Error:", ok)
					if ct > 5 {
						parsed <- nil
						return
					}
					fmt.Println("retrying - attempt #" + strconv.Itoa(ct))
					time.Sleep(200 * time.Millisecond)
					continue
				}
				defer res.Body.Close()
				break
    		}
    		var got moviesResponse
    		json.NewDecoder(res.Body).Decode(&got)
    		if got.V == nil {
    			parsed <- nil
    			return
    		}
    		got.V["sources"] = []ItemSource{}
    		parsed <- got.V
    	} (imdb_id)
    }

    // Populate result array from channel
    count := 0
    for ; count < len(ids); {
    	on := <- parsed
    	count += 1
    	if on != nil {
    		ret = append(ret, on)
    	}
    }

    // Sort before returning
    countReducer := func (count float64) float64 {
		return math.Log(count) / math.Log(50) // log base 50
	}
    sort.Slice(ret[:], func(i, j int) bool {
    	// descending sort
    	x, y := ret[i], ret[j]
    	if tmp, ok := x["unreleased"].(bool); ok && tmp {
    		return false;
    	}
    	if tmp, ok := y["unreleased"].(bool); ok && tmp {
    		return true;
    	}
    	a, ok := x["imdb_rating"].(float64)
    	if !ok {
    		return false;
    	}
    	b, ok := y["imdb_rating"].(float64)
    	if !ok {
    		return true;
    	}
		a *= countReducer(x["imdb_rating_count"].(float64))
		b *= countReducer(y["imdb_rating_count"].(float64))
		return (a > b)
    })
    return ret, err
}

const (
	MoviePopularUrl = "/movies/popular"
	MovieWatchedUrl = "/movies/watched/yearly"
	MovieTrendingUrl = "/movies/trending"
	MovieSearchTextUrl = "/search/movie"
	MovieWatchlistGetUrl = "/sync/watchlist/movie"
	WatchlistAddUrl = "/sync/watchlist"
	HistoryGetUrl = "/sync/history"
	HistoryAddUrl = "/sync/history"
	PlaybackGetUrl = "/sync/playback/movies"
	ScrobbleStartUrl = "/scrobble/start"
	ScrobblePauseUrl = "/scrobble/pause"
	ScrobbleStopUrl = "/scrobble/stop"
)

func newTraktRequest(uri string) (*trakt.Request, error) {
	req, err := traktClient.NewRequest(uri)
	if err != nil {
		return nil, err
	}
	delete(req.Query, "extended")
	return req, err
}

func traktPaginateUrl(url string, page, limit int) string {
	return (url + "?page=" + strconv.Itoa(page) + "&limit=" + strconv.Itoa(limit))
}

func traktRequestGet(path string) (interface{}, error) {
	url := configuration.TraktBaseUrl + path
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer " + configuration.TraktAccessToken)
	req.Header.Set("trakt-api-version", "2")
	req.Header.Set("trakt-api-key", configuration.TraktClientId)
	res, err := netClient.Do(req)
	if err != nil {
		return nil, err
	}
	res_bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var ret interface{}
	json.Unmarshal(res_bytes, &ret)
	return ret, nil
}

func mapToField(obj []map[string]interface{}, field string) ([]map[string]interface{}) {
	for i, on := range obj {
		have, ok := on[field].(map[string]interface{})
		if ok {
			obj[i] = have
		}
	}
	return obj
}

func deDup(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)
	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}
	return u
}

func filterTraktIds(output []map[string]interface{}) ([]string) {
	var ids []string
	for _, v := range output {
		tmp, ok := (v["ids"].(map[string]interface{}))
		if !ok {
			continue
		}
		id, ok := tmp["imdb"].(string)
		if ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func executeParallelResolution(ids []string, load_balancer_addr string) ([]map[string]interface{}, error) {
	var output []map[string]interface{}
	if len(ids) == 0 {
		return nil, errors.New("No ID's to resolve")
	}

	/* Generate the request */
	to_send := new(bytes.Buffer)
	json.NewEncoder(to_send).Encode(map[string]interface{}{
		"q": map[string]interface{}{
			"type": "resolveParallel",
			"data": map[string]interface{}{
				"ids": ids,
			},
		},
	})
	posting_url := fmt.Sprintf("http://%s/movies", load_balancer_addr)
	//fmt.Println(to_send)

	/* Execute the request */
	for ct := 0; ; ct += 1 {
		res, err := netClient.Post(
			posting_url,
			"application/json; charset=utf-8",
			to_send,
		)
		if err != nil {
			fmt.Println("Error:", err)
			if ct > 5 {
				return nil, err
			}
			fmt.Println("Retrying - error #" + strconv.Itoa(ct))
			continue
		}

		/* Parse the response */
		var got moviesResponse
		req_resp, _ := ioutil.ReadAll(res.Body)
		if err = json.Unmarshal([]byte(req_resp), &got); err != nil {
			return nil, errors.New(fmt.Sprintf("err: %s; body: %s", err, string(req_resp)))
		}
		interface_arr, ok := got.V["resolved"].([]interface{})
		if !ok {
			fmt.Println("Could not resolve, retrying:", posting_url)
			fmt.Println(got)
			if ct > 5 {
				fmt.Println("Giving up")
				return make([]map[string]interface{}, 0), nil
			}
			time.Sleep(500 * time.Millisecond)
			//return nil, errors.New("Resolution unsuccessful")
			continue
		}

		/* Convert output to desired format */
		for _, elem := range interface_arr {
			output = append(output, elem.(map[string]interface{}))
		}

		/* Return parsed response */
		return output, nil
	}
}

func (movieData) GetRecommendedMovies(extension int, load_balancer_addr string) (ret []map[string]interface{}, err error) {
	var output []map[string]interface{}
	var tmp []map[string]interface{}
	if extension < 0 {
		extension = 0
	}

	/* Get top Trakt.tv movies */
	req, err := newTraktRequest(traktPaginateUrl(MoviePopularUrl, 1 + extension, 25))
	req.Get(&tmp)
	output = append(output, tmp...)

	req, err = newTraktRequest(traktPaginateUrl(MovieWatchedUrl, 1 + extension, 25))
	tmp = nil
	req.Get(&tmp)
	output = append(output, mapToField(tmp, "movie")...)

	req, err = newTraktRequest(traktPaginateUrl(MovieTrendingUrl, 1 + extension, 25))
	tmp = nil
	req.Get(&tmp)
	output = append(output, mapToField(tmp, "movie")...)

	/* Map output to IMDB id's */
	ids := filterTraktIds(output)
	ids = deDup(ids)
	output = nil
	fmt.Println(ids)

	// TODO: Get Taste.io recommendations as well

	/* Resolve ID's in parallel */
	output, err = executeParallelResolution(ids, load_balancer_addr)

	/* Return output */
	return output, err
}

func searchTraktMovies(keyword string, item_type string) ([]map[string]interface{}, error) {
	var tmp []map[string]interface{}

	/* Execute Trakt.tv search for keyword */
	var base_url string
	if item_type == "movie" {
		base_url = MovieSearchTextUrl
	} else {
		return nil, errors.New("Unknown item type")
	}
	req, err := newTraktRequest(traktPaginateUrl(base_url, 1, 25) + "&query=" + keyword)
	req.Get(&tmp)
	//fmt.Println(tmp)
	return mapToField(tmp, "movie"), err
}

func getTraktWatchlist(item_type string) ([]map[string]interface{}, error) {
	var tmp []map[string]interface{}

	/* Execute Trakt.tv watchlist retrieval */
	var base_url string
	if item_type == "movie" {
		base_url = MovieWatchlistGetUrl
	} else {
		return nil, errors.New("Unknown item type")
	}
	req, err := newTraktRequest(traktPaginateUrl(base_url, 1, 50000))
	req.Get(&tmp)

	return mapToField(tmp, "movie"), err
}

func cacheSources(sources map[string][]ItemSource) {
	for imdb_id, sourceArr := range sources {
		if sourceArr == nil {
			continue;
		}

		/* Retrieve existing cached values, if applicable */
		var existing []ItemSource
		cached, ok := cache.Get([]byte(ITEM_KEY_ID + imdb_id))
		if ok == nil && cached != nil {
			buf := bytes.NewBuffer(cached)
			dec := gob.NewDecoder(buf)
			v := dec.Decode(&existing)
			if v != nil {
				panic(v)
			}
		}

		/* Gracefully merge current and cached item sources */
		merged := make(map[string]ItemSource)
		for _, elem := range sourceArr {
			merged[elem.Url] = elem
		}
		for _, elem := range existing {
			if _, ok := merged[elem.Url]; ok {
				continue;
			}
			merged[elem.Url] = elem
		}

		sourceArr = nil
		for _, v := range merged {
			sourceArr = append(sourceArr, v)
		}
		merged = nil

		/* Save merged array to cache */
		source_bytes, ok := GetBytes(sourceArr)
		cache.Set([]byte(ITEM_KEY_ID + imdb_id), source_bytes, /*1 hour=*/1 * 60 * 60)
	}
}

func (movieData) SearchForItem(opts map[string]interface{}, load_balancer_addr string) ([]map[string]interface{}, error) {
	var tmp, output []map[string]interface{}
	var imdb_ids []string
	sources := make(map[string][]ItemSource)
	var err error

	/* Retrieve matches */
	if imdb_id, ok := opts["id"].(string); ok {
		sources[imdb_id], err = SearchSourcesParallel(opts)

		/* Cache sources only if searching for an item directly */
		cacheSources(sources)
	} else if keyword, ok := opts["keyword"].(string); ok {
		/* Search Trakt.tv */
		tmp, err = searchTraktMovies(keyword, "movie")
		if err != nil {
			return nil, err
		}
		imdb_ids = append(imdb_ids, filterTraktIds(tmp)...)

		/* Search sources */
		source_results, err := SearchSourcesParallel(opts)
		if err != nil {
			return nil, err
		}
		for _, elem := range source_results {
			sources[elem.ImdbCode] = append(sources[elem.ImdbCode], elem)
		}
	}

	/* Add ID's from sources */
	for _, sourceItems := range sources {
		for _, item := range sourceItems {
			imdb_ids = append(imdb_ids, item.ImdbCode)
		}
	}

	/* Resolve matches */
	imdb_ids = deDup(imdb_ids)
	if len(imdb_ids) > 0 {
		output, err = executeParallelResolution(imdb_ids, load_balancer_addr)
	} else {
		output = make([]map[string]interface{}, 0)
	}

	/* Correlate resolved items and sources */
	for idx := 0; idx < len(output); idx += 1 {
		cur_imdb_code, _ := output[idx]["imdb_code"].(string)
		if have, ok := sources[cur_imdb_code]; ok {
			output[idx]["sources"] = have
		}
	}

	/* Return matches */
	return output, err
}

func (movieData) GetWatchlist(load_balancer_addr string) ([]string, error) {
	var tmp []map[string]interface{}
	var imdb_ids []string
	var err error

	/* Retrieve movie watchlist */
	for ct := 0;; ct += 1 {
		tmp, err = getTraktWatchlist("movie")
		if err != nil {
			if ct > 5 {
				return nil, err
			}
			fmt.Println("Retrying - error:", err)
			continue
		}

		/* Filter IMDB id's from result */
		imdb_ids = filterTraktIds(tmp)

		/* Resolve matches */
		imdb_ids = deDup(imdb_ids)

		if len(imdb_ids) == 0 {
			if ct > 5 {
				return nil, errors.New("Gave up on Trakt watchlist retrieval")
			}
		} else {
			break
		}
	}

	/* Return matches */
	return imdb_ids, nil
}

func (movieData) AddToWatchlist(item_type string, item_id string) (map[string]interface{}, error) {
	var tmp map[string]interface{}
	var video_obj map[string]interface{}

	/* Execute Trakt.tv watchlist insertion */
	base_url := WatchlistAddUrl
	if item_type == "movie" {
		video_obj = map[string]interface{}{
			"movies": []map[string]interface{}{
				{"ids": map[string]interface{}{
					"imdb": item_id,
				}},
			},
		}
	} else {
		return nil, errors.New("Unknown item type")
	}
	req, err := newTraktRequest(base_url)
	req.Post(video_obj, &tmp)

	return tmp, err
}

func (movieData) GetWatchHistory(load_balancer_addr string) ([]string, error) {
	var tmp []map[string]interface{}
	var imdb_ids []string
	var ids []string

	/* Execute Trakt.tv history retrieval */
	for ct := 0;; ct += 1 {
		req, err := newTraktRequest(traktPaginateUrl(HistoryGetUrl, 1, 50000))
		if err != nil {
			if ct > 5 {
				return nil, err
			}
			continue
		}
		req.Get(&tmp)

		/* Filter for IMDB id's */
		imdb_ids = append(imdb_ids, filterTraktIds(mapToField(tmp, "movie"))...)
		imdb_ids = append(imdb_ids, filterTraktIds(mapToField(tmp, "show"))...)

		ids = deDup(imdb_ids)

		if len(ids) > 0 {
			break
		}
		if ct > 5 {
			return nil, errors.New("Gave up for Trakt history retrieval")
		}
	}

	return ids, nil
}

func (movieData) AddWatchHistory(item_type string, item_id string) (map[string]interface{}, error) {
	var tmp map[string]interface{}
	var video_obj map[string]interface{}

	/* Execute Trakt.tv history insertion */
	var base_url string
	if item_type == "movie" {
		base_url = HistoryAddUrl
		video_obj = map[string]interface{}{
			"movies": []map[string]interface{}{
				{"ids": map[string]interface{}{
					"imdb": item_id,
				}},
			},
		}
	} else {
		return nil, errors.New("Unknown item type")
	}
	req, err := newTraktRequest(base_url)
	req.Post(video_obj, &tmp)

	return tmp, err
}

func (movieData) UpdateScrobbleStatus(imdb_code string, progress float64, state string) (map[string]interface{}, error) {
	var tmp map[string]interface{}
	var video_obj map[string]interface{}

	/* Execute Trakt.tv history insertion */
	var base_url string
	if state == "started" {
		base_url = ScrobbleStartUrl
	} else if state == "paused" {
		base_url = ScrobblePauseUrl
	} else if state == "stopped" {
		base_url = ScrobbleStopUrl
	}
	video_obj = map[string]interface{}{
		"movie": map[string]interface{}{
			"ids": map[string]interface{}{
				"imdb": imdb_code,
			},
		},
		"progress": progress,
	}
	for ct := 0;; ct += 1 {
		req, err := newTraktRequest(base_url)
		if err != nil {
			return nil, err
		}
		req.Post(video_obj, &tmp)
		if tmp != nil {
			break
		}
		if ct > 5 {
			return nil, errors.New("Gave up for Trakt scrobble update")
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.Encode(tmp)

	return tmp, nil
}

func (movieData) GetPlaybackScrobbles(load_balancer_addr string) ([]map[string]interface{}, error) {
	tmp := make([]map[string]interface{}, 0)

	/* Execute Trakt.tv history retrieval */
	for ct := 0;; ct += 1 {
		req, err := traktRequestGet(PlaybackGetUrl)
		if err != nil {
			if ct > 5 {
				return nil, err
			} else {
				continue
			}
		}
		tmp_arr := req.([]interface{})
		for _, on := range tmp_arr {
			tmp = append(tmp, on.(map[string]interface{}))
		}

		// enc := json.NewEncoder(os.Stdout)
		// enc.Encode(tmp_arr)
		break
	}

	return tmp, nil
}

func (md movieData) GetItem(id string, load_balancer_addr string) (map[string]interface{}, error) {
	// Look up by ID, fill in sources from cache, return
	// If not cached, silently call SearchForItem and return item of specified ID
	existing := make([]ItemSource, 0)
	item_key := []byte(ITEM_KEY_ID + id)

	/* Check if cached */
	cached, ok := cache.Get(item_key)
	if ok != nil || cached == nil {
		/* If not cached, search for sources and return the item and its sources */
		outp, err := md.SearchForItem(map[string]interface{}{
			"id": id,
		}, load_balancer_addr)
		if err != nil {
			return nil, err
		}

		for _, elem := range outp {
			if elem["imdb_code"].(string) == id {
				return elem, nil
			}
		}
	} else {
		/* If cached, parse cache */
		buf := bytes.NewBuffer(cached)
		dec := gob.NewDecoder(buf)
		v := dec.Decode(&existing)
		if v != nil {
			panic(v)
		}
	}

	/* Resolve item */
	output, err := executeParallelResolution([]string{id}, load_balancer_addr)
	if err != nil {
		return nil, err
	}
	if len(output) != 1 {
		return nil, errors.New(fmt.Sprintf("Expected 1 resolved, got %d", len(output)))
	}

	/* Fill in sources and return requested item */
	output[0]["sources"] = existing
	return output[0], nil
}














