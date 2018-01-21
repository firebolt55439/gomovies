package main

import (
	"time"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"encoding/json"
	"errors"
	"strings"
	"math"
	"net/http"
	
	"github.com/mitchellh/mapstructure"
)

type ItemSourceTV struct {
	Season int `json:"season"` // season #
	Episode int `json:"episode"` // episode #
}

type ItemSource struct {
	ImdbCode string `json:"imdb_code"` // IMDb code of sourced item
	Quality string `json:"quality"` // 3D, 720p, etc.
	Size string `json:"size"` // humanized size string
	Filename string `json:"filename"` // name of file
	Url string `json:"url"` // file URL
	SourceCount int `json:"sources"` // file source count
	ClientCount int `json:"clients"` // file client count
	SourceHostname string `json:"source"` // source hostname
	TV *ItemSourceTV `json:"tv,omitempty"` // TV information, if applicable
}

type SourceApiStorage struct {
	Token string `json:"token"`
	ExpiryTime time.Time
}

type SourceA struct {
	SourceApiBaseUrl string `mapstructure:"base_url"`
	SourceApiClientId string `mapstructure:"client_id"`
	SourceApiSortKey string `mapstructure:"sort_key"`
	SourceApiResultLimit int `mapstructure:"result_limit"`
	SourceApiSourceKey string `mapstructure:"source_key"`
	SourceApiClientKey string `mapstructure:"client_key"`
	SourceApiHostname string `mapstructure:"hostname"`
}

type SourceB struct {
	BaseUrl string `mapstructure:"base_url"`
	OrderBy int `mapstructure:"order_by"`
	PageNumber int `mapstructure:"page_number"`
	LinkStartKeyword string `mapstructure:"link_start_keyword"`
	ValidityCheckKeywords []interface{} `mapstructure:"validity_check_keywords"`
	SourceApiHostname string `mapstructure:"hostname"`
}

var sourceApiStorage SourceApiStorage

func getTokenIfNecessary(configuration SourceA) (string, error) {
	/* Check cached token for validity, and return if valid */
	expiry_time := sourceApiStorage.ExpiryTime
	if !expiry_time.IsZero() && (expiry_time.Sub(time.Now().Local()) >= (time.Duration(10) * time.Second)) {
		return sourceApiStorage.Token, nil
	}
	
	/* Otherwise, acquire a new token */
	target_url := fmt.Sprintf("%s?get_token=get_token", configuration.SourceApiBaseUrl)
	res, err := netClient.Get(
		target_url,
	)
	if err != nil {
		fmt.Println("Error:", err)
		return "", err
	}
	defer res.Body.Close()
	
	/* Parse the response */
	var got SourceApiStorage
	json.NewDecoder(res.Body).Decode(&got)
	
	/* Cache parsed response */
	sourceApiStorage.Token = got.Token
	sourceApiStorage.ExpiryTime = time.Now().Local().Add(15 * time.Minute)
	
	/* Return parsed response */
	return got.Token, nil
}

func detectTitleQuality(title string) (string, error) {
	qualityTitleMap := map[string]string {
		"720": "720p",
		"1080": "1080p",
		"3D": "3D",
		"TV HD": "HD",
	}
	for _, elem := range configuration.TitleQualityHDKeywords {
		qualityTitleMap[elem] = "HD"
	}
	
	for k, v := range qualityTitleMap {
		if strings.Contains(title, k) {
			return v, nil
		}
	}
	
	return "", errors.New("Could not detect quality")
}

func bytesToSize(bytes float64) (string) {
	i := math.Floor(math.Log(bytes) / math.Log(1024))
	prefixArr := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	return fmt.Sprintf("%.2f %s", (bytes / math.Pow(1024, i)), prefixArr[int(i)])
}

var sources = []func(map[string]interface{}, SourceConfig) ([]ItemSource, error){
	func (opts map[string]interface{}, conf SourceConfig) (ret []ItemSource, err error) {
		var configuration SourceA
		if err := mapstructure.Decode(conf, &configuration); err != nil {
			return nil, err
		}
		ret = make([]ItemSource, 0)
		
		/* Generate search parameters */
		token, err := getTokenIfNecessary(configuration)
		if err != nil {
			return nil, err
		}
		searchParams := url.Values{
			"sort": {configuration.SourceApiSortKey},
			"limit": {strconv.Itoa(configuration.SourceApiResultLimit)},
			"format": {"json_extended"},
			"app_id": {configuration.SourceApiClientId},
			"mode": {"search"},
			"token": {token},
		}
		searchParams.Set("min_" + configuration.SourceApiSortKey, strconv.Itoa(3))
		
		if imdb_id, ok := opts["id"].(string); ok {
			searchParams.Set("search_imdb", imdb_id)
		} else if keyword, ok := opts["keyword"].(string); ok {
			searchParams.Set("search_string", keyword)
		}
		
		/* Continue retrying request up to threshold */
		target_url := fmt.Sprintf("%s?%s", configuration.SourceApiBaseUrl, searchParams.Encode())
		var results []map[string]interface{}
		
		for attempt := 1; attempt <= 5; attempt += 1{
			/* Generate and execute request */
			res, err := netClient.Get(
				target_url,
			)
			if err != nil {
				fmt.Println("Error:", err)
				return nil, err
			}
			defer res.Body.Close()
		
			/* Parse and validate response */
			var got map[string]interface{}
			json.NewDecoder(res.Body).Decode(&got)
			/*
			pretty_printed, _ := json.MarshalIndent(got, "", "  ")
			slice_until := int(math.Min(300, float64(len(pretty_printed))))
			fmt.Println(string(pretty_printed)[0:slice_until])
			*/
			
			if error, ok := got["error"]; ok {
				if strings.Contains(error.(string), "No results found") {
					return ret, nil
				}
				if strings.Contains(error.(string), "Cant find imdb") {
					return ret, nil
				}
				fmt.Println(fmt.Sprintf("Retrying (error %s)", error))
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			
			var resultsArr []interface{}
			for _, v := range got {
				resultsArr = v.([]interface{}) // first value
				break
			}
			for _, elem := range resultsArr {
				results = append(results, elem.(map[string]interface{}))
			}
			
			break
		}
		
		if len(results) == 0 {
			return nil, errors.New("Could not complete search request!")
		}
		
		/* Convert response to desired format */
		qualityCategoryMap := map[string]string {
			"720": "720p",
			"1080": "1080p",
			"3D": "3D",
			"TV HD": "HD",
		}
		for _, on := range results {
			if _, ok := on["episode_info"]; !ok {
				continue
			}
			episode_info, ok := on["episode_info"].(map[string]interface{})
			if !ok {
				continue
			}
			if _, ok := episode_info["imdb"]; !ok {
				continue
			}
			
			quality, have_quality := "(unknown)", false
			category := on["category"].(string)
			title := on["title"].(string)
			for k, v := range qualityCategoryMap {
				if strings.Contains(category, k) || strings.Contains(title, k) {
					have_quality = true
					quality = v
					break
				}
			}
			if !have_quality {
				var tmp_err error
				quality, tmp_err = detectTitleQuality(title)
				have_quality = (tmp_err == nil)
			}
			if !have_quality {
				quality = "SD"
				fmt.Println(fmt.Sprintf(
					"Warning: Could not detect quality for title %s and category %s",
					on["title"],
					on["category"],
				))
			}
			
			humanizedSize := bytesToSize(on["size"].(float64))
			
			ret = append(ret, ItemSource{
				ImdbCode: episode_info["imdb"].(string),
				Quality: quality,
				Size: humanizedSize,
				Filename: on["title"].(string),
				Url: on["download"].(string),
				SourceCount: int(on[configuration.SourceApiSourceKey].(float64)),
				ClientCount: int(on[configuration.SourceApiClientKey].(float64)),
				SourceHostname: configuration.SourceApiHostname,
			})
			
			if _, ok := episode_info["seasonnum"]; ok {
				season_num, _ := strconv.Atoi(episode_info["seasonnum"].(string))
				ep_num, _ := strconv.Atoi(episode_info["epnum"].(string))
				ret[len(ret) - 1].TV = &ItemSourceTV{
					Season: season_num,
					Episode: ep_num,
				}
			}
		}
		
		/* Return converted response */
		return ret, err
	},
	
	func (opts map[string]interface{}, conf SourceConfig) (ret []ItemSource, err error) {
		var configuration SourceB
		if err := mapstructure.Decode(conf, &configuration); err != nil {
			return nil, err
		}
		ret = make([]ItemSource, 0)
		
		/* Generate url */
		search_term := ""
		if imdb_id, ok := opts["id"].(string); ok {
			search_term = imdb_id
		} else if _, ok := opts["keyword"].(string); ok {
			return nil, errors.New("Keyword search is not supported by this source")
		}
		
		form_url := fmt.Sprintf(
			"%s/s/?q=%s&category=%d&page=%d&orderby=%d",
			configuration.BaseUrl,
			search_term,
			0, /* TODO */
			configuration.PageNumber,
			configuration.OrderBy,
		)
		fmt.Println(form_url)
		
		// TODO: Race to first parsed response via proxy list
		
		/* Download page */
		var resp (*http.Response)
		for ct := 0;; ct += 1 {
			resp, err = netClient.Get(form_url)
			if err != nil {
				if ct > 5 {
					return nil, err
				}
				continue
			}
			break
		}
		bytes, _ := ioutil.ReadAll(resp.Body)
		body := string(bytes)
		
		/* Parse search results */
		table_arr := strings.Split(body, "id=\"searchResult\"")
		if len(table_arr) <= 1 {
			return nil, errors.New("Could not find search result table")
		}
		table := strings.Split(table_arr[1], "</table>")[0]
		
		rows := strings.Split(table, "<tr")[1:]
		for _, on := range rows {
			/* Validity check */
			passed := true
			for _, itf := range configuration.ValidityCheckKeywords {
				if kw, ok := itf.(string); ok {
					/* A string means that the given specific keyword is required */
					if !strings.Contains(on, kw) {
						//fmt.Println(fmt.Sprintf("Failed on keyword %s", kw))
						passed = false
						break
					}
				} else if or_arr, ok := itf.([]string); ok {
					/* An array means any of the keywords works */
					or_passed := false
					for _, kw := range or_arr {
						if strings.Contains(on, kw) {
							or_passed = true
							break
						}
					}
					if !or_passed {
						passed = false
						break
					}
				}
			}
			if !passed {
				continue
			}
			
			link := strings.Split(strings.Split(on, configuration.LinkStartKeyword)[1], "\"")[0]
			link = configuration.LinkStartKeyword + link
			
			size := strings.Split(strings.Split(on, ", Size ")[1], ",")[0]
			size = strings.Replace(size, "&nbsp;", " ", -1)
			
			filename := strings.Split(strings.Split(on, "detName\">")[1], "\">")[1]
			filename = strings.Split(filename, "</a>")[0]
			
			quality, ok := detectTitleQuality(filename)
			if ok != nil {
				quality = "SD"
			}
			
			counts_area := strings.SplitN(on, "td align=\"right\">", 2)
			counts_area = strings.Split(counts_area[1], "\"right\">")
			source_count, _ := strconv.Atoi(strings.Split(counts_area[0], "</td>")[0])
			clients_count, _ := strconv.Atoi(strings.Split(counts_area[1], "</td>")[0])
			
			ret = append(ret, ItemSource{
				ImdbCode: search_term,
				Quality: quality,
				Size: size,
				Filename: filename,
				Url: link,
				SourceCount: source_count,
				ClientCount: clients_count,
				SourceHostname: configuration.SourceApiHostname,
			})
		}
		
		return ret, err
	},
}

func SearchSourcesParallel(opts map[string]interface{}) (ret []ItemSource, err error) {
	defer func() {
        if r := recover(); r != nil {
            err = errors.New(fmt.Sprintf("SearchSourcesParallel was panicking, recovered value: %v (%s)", r, identifyPanic()))
        }
    }()
    err = nil
    parsed := make(chan []ItemSource, len(sources))

    /* Search sources in parallel */
    source_idx := 0
    for _, fn := range sources {
    	go func(fn func(map[string]interface{}, SourceConfig) ([]ItemSource, error), conf SourceConfig) {
    		res, ok := fn(opts, conf)
    		if ok != nil {
    			fmt.Println("Warning:", ok)
    			parsed <- nil
    		} else {
    			parsed <- res
    		}
    	} (fn, configuration.Sources[source_idx])
    	source_idx += 1
    }

    /* Populate result array from channel */
    count := 0
    for ; count < len(sources); {
    	on := <- parsed
    	count += 1
    	if on != nil {
    		ret = append(ret, on...)
    	}
    }

    /* Return result */
    return ret, err
}















