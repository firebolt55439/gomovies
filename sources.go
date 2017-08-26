package main

import (
	"time"
	"fmt"
	"net/url"
	"strconv"
	"encoding/json"
	"errors"
	"strings"
	"math"
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
	TV *ItemSourceTV `json:"tv,omitempty"` // TV information, if applicable
}

type SourceApiStorage struct {
	Token string `json:"token"`
	ExpiryTime time.Time
}

var sourceApiStorage SourceApiStorage

func getTokenIfNecessary() (string, error) {
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
	defer res.Body.Close()
	if err != nil {
		fmt.Println("Error:", err)
		return "", err
	}
	
	/* Parse the response */
	var got SourceApiStorage
	json.NewDecoder(res.Body).Decode(&got)
	
	/* Cache parsed response */
	sourceApiStorage.Token = got.Token
	sourceApiStorage.ExpiryTime = time.Now().Local().Add(15 * time.Minute)
	
	/* Return parsed response */
	return got.Token, nil
}

func bytesToSize(bytes float64) (string) {
	i := math.Floor(math.Log(bytes) / math.Log(1024))
	prefixArr := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	return fmt.Sprintf("%.2f %s", (bytes / math.Pow(1024, i)), prefixArr[int(i)])
}

var sources = []func(map[string]interface{}) ([]ItemSource, error){
	func (opts map[string]interface{}) (ret []ItemSource, err error) {
		/* Generate search parameters */
		token, err := getTokenIfNecessary()
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
		
		for {
			/* Generate and execute request */
			res, err := netClient.Get(
				target_url,
			)
			defer res.Body.Close()
			if err != nil {
				fmt.Println("Error:", err)
				return nil, err
			}
		
			/* Parse and validate response */
			var got map[string]interface{}
			json.NewDecoder(res.Body).Decode(&got)
			pretty_printed, _ := json.MarshalIndent(got, "", "  ")
			fmt.Println(string(pretty_printed)[0:300])
			
			if error_code, ok := got["error_code"].(int); ok {
				fmt.Println(fmt.Sprintf("Retrying (error code %d)", error_code))
				time.Sleep(1 * time.Second)
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
		
		/* Convert response to desired format */
		qualityCategoryMap := map[string]string {
			"720": "720p",
			"1080": "1080p",
			"3D": "3D",
			"TV HD": "HD",
		}
		qualityTitleMap := make(map[string]string)
		for _, elem := range strings.Split(configuration.SourceApiHdTitle, ",") {
			qualityTitleMap[elem] = "HD"
		}
		for _, on := range results {
			if _, ok := on["episode_info"]; !ok {
				continue
			}
			episode_info := on["episode_info"].(map[string]interface{})
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
				for k, v := range qualityTitleMap {
					if strings.Contains(title, k) {
						have_quality = true
						quality = v
						break
					}
				}
			}
			if !have_quality {
				fmt.Println(fmt.Sprintf(
					"Warning: Could not detect quality for title %s and category %s",
					on["title"],
					on["category"],
				))
				continue
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
    for _, fn := range sources {
    	go func(fn func(map[string]interface{}) ([]ItemSource, error)) {
    		res, ok := fn(opts)
    		if ok != nil {
    			fmt.Println("Warning:", ok)
    			parsed <- nil
    		} else {
    			parsed <- res
    		}
    	} (fn)
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















