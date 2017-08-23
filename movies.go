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
)

type MovieData interface {
	ScrapeImdb(id string) (map[string]interface{}, error)
	ResolveParallel(ids []string, load_balancer_addr string) ([]map[string]interface{}, error)
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
	parsed["poster"] = poster
	
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
    		//ret = make(map[string]interface{})
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










