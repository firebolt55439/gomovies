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
	
	"github.com/coocood/freecache"
	"github.com/42minutes/go-trakt"
)

type MovieData interface {
	/* IMDB metadata resolution */
	ScrapeImdb(id string) (map[string]interface{}, error)
	ResolveParallel(ids []string, load_balancer_addr string) ([]map[string]interface{}, error)
	
	/* Trakt.tv and taste.io integration */
	GetRecommendedMovies(extension int, load_balancer_addr string) ([]map[string]interface{}, error)
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

/* Interface functions */
const IMDB_KEY_ID string = "imdbKeyId-";

var cache *freecache.Cache = freecache.NewCache(10 * 1024 * 1024)
var netClient = &http.Client{
	Timeout: time.Second * 3,
}

func (movieData) ScrapeImdb(id string) (parsed map[string]interface{}, err error) {
	defer func() {
        if r := recover(); r != nil {
            err = errors.New(fmt.Sprintf("was panicking, recovered value: %v", r))
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
	resp, _ := netClient.Get(imdb_url)
	bytes, _ := ioutil.ReadAll(resp.Body)
	body := string(bytes)
	//fmt.Println(body[0:200])
	resp.Body.Close()
	
	// Validity check
	if strings.Index(body, "div class=\"title_wrapper\">") == -1 {
		return nil, errors.New("No item matched for given IMDB id")
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
	summary = strings.TrimSpace(summary)
	summary = html.UnescapeString(summary)
	parsed["summary"] = summary
	
	// TV Show Detection
	is_tv_show := strings.Index(mpaa_rating, "TV") != -1
	parsed["is_tv_show"] = is_tv_show
	
	// Cache result
	parsed_bytes, ok := GetBytes(parsed)
	cache.Set([]byte(IMDB_KEY_ID + id), parsed_bytes, /*never expires*/0)
	
	// Return gathered data
	return parsed, err
}

func (movieData) ResolveParallel(ids []string, load_balancer_addr string) (ret []map[string]interface{}, err error) {
	defer func() {
        if r := recover(); r != nil {
            err = errors.New(fmt.Sprintf("was panicking, recovered value: %v", r))
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
    		res, ok := netClient.Post(
    			posting_url,
    			"application/json; charset=utf-8",
    			to_send,
    		)
    		if ok != nil {
    			fmt.Println("Error:", ok)
    			parsed <- nil
    			return
    		}
    		var got moviesResponse
    		json.NewDecoder(res.Body).Decode(&got)
    		got.V["sources"] = []string{}
    		got.V["title"] = fmt.Sprintf("%s (%.0f)", got.V["title"], got.V["year"])
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
    	a, b := x["imdb_rating"].(float64), y["imdb_rating"].(float64)
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

func mapToField(obj []map[string]interface{}, field string) ([]map[string]interface{}) {
	for i, on := range obj {
		//fmt.Println(on[field])
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
	var ids []string
	for _, v := range output {
		id, ok := (v["ids"].(map[string]interface{}))["imdb"].(string)
		if ok {
			ids = append(ids, id)
		}
	}
	ids = deDup(ids)
	output = nil
	fmt.Println(ids)
	
	// TODO: Get Taste.io recommendations as well
	
	/* Resolve ID's in parallel */
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
	res, err := netClient.Post(
		posting_url,
		"application/json; charset=utf-8",
		to_send,
	)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}
	var got moviesResponse
	json.NewDecoder(res.Body).Decode(&got)
	interface_arr := got.V["resolved"].([]interface{})
	
	/* Convert output to desired format */
	for _, elem := range interface_arr {
		output = append(output, elem.(map[string]interface{}))
	}
	
	/* Return output */
	return output, err
}




















