package main

import (
	"fmt"
	"time"
	"strconv"
	"sort"
)

// TODO: Persist download tags using disk

type DownloadItem struct {
	Imdb_id string `json:"imdb_id"`
	Source string `json:"source"` /* "disk", "oauth", etc. */
	Name string `json:"name"` /* name of item */
	Cloud_id string `json:"id"` /* oauth cloud item id, if/a */
	Progress float64 `json:"progress,omitempty"` /* progress of current operation */
	Time_started int64 `json:"time_started,omitempty"` /* unix timestamp in seconds of start time, if/a */
	Size int64 `json:"size"` /* size of file, -1 if unknown */

	IsDownloadingCloud bool `json:"isDownloadingCloud"` /* true if cloud is downloading item */
	HasDownloadedCloud bool `json:"hasDownloadedCloud"` /* true if cloud has downloaded item */

	IsDownloadingClient bool `json:"isDownloadingClient"` /* true if client is downloading item */
	HasDownloadedClient bool `json:"hasDownloadedClient"` /* true if client has downloaded item */

	IsUploadingClient bool `json:"isUploadingClient"` /* true if client is uploading item */
	HasUploadedClient bool `json:"hasUploadedClient"` /* true if client has uploaded item */
}

type DownloadsInterface interface {
	RetrieveDownloads() ([]DownloadItem)
	RegisterOAuthDownloadStart(imdb_id string, cloud_id string, name string) (error)
	RefreshDownloadStates(states []interface{}) (error)
}

type Downloads struct {
	pool []*DownloadItem
}

func (dl *Downloads) RetrieveDownloads() ([]DownloadItem) {
	var pool []DownloadItem
	for _, on := range dl.pool {
		pool = append(pool, *on)
	}
	return pool
}

func (dl *Downloads) RegisterOAuthDownloadStart(imdb_id string, cloud_id string, name string) (error) {
	var err error = nil
	dl.pool = append(dl.pool, &DownloadItem{
		Imdb_id: imdb_id,
		Source: "oauth",
		Cloud_id: cloud_id,
		Name: name,
		Progress: -1.0,
		Size: -1,
		Time_started: time.Now().Unix(),
		IsDownloadingCloud: true,
		HasDownloadedCloud: false,
		IsDownloadingClient: false,
		HasDownloadedClient: false,
		IsUploadingClient: false,
		HasUploadedClient: false,
	})
	return err
}

func contains(s []string, e string) bool {
    for _, a := range s {
        if a == e {
            return true
        }
    }
    return false
}

func (dl *Downloads) RefreshDownloadStates(states []interface{}) (error) {
	var err error = nil
	var updatedIds []string
	for _, on_m := range states {
		on := on_m.(map[string]interface{})
		id := strconv.Itoa(int(on["id"].(float64)))
		updatedIds = append(updatedIds, id)
		foundItem := &DownloadItem{
			Imdb_id: "",
			Source: "oauth",
			Name: on["name"].(string),
			Cloud_id: id,
			Size: -1,
			IsDownloadingCloud: false,
			HasDownloadedCloud: false,
			IsDownloadingClient: false,
			HasDownloadedClient: false,
			IsUploadingClient: false,
			HasUploadedClient: false,
		}
		didFindItem := false
		for _, tmp := range dl.pool {
			if tmp.Source == "oauth" && tmp.Cloud_id == id {
				foundItem = tmp
				didFindItem = true
				break
			}
		}
		if tmp_prog, ok := on["progress"]; ok {
			foundItem.Progress, _ = strconv.ParseFloat(tmp_prog.(string), /*bitsize=*/64)
			foundItem.IsDownloadingCloud = true
			foundItem.HasDownloadedCloud = false
		} else {
			foundItem.IsDownloadingCloud = false
			foundItem.HasDownloadedCloud = true
		}
		foundItem.Size, _ = on["size"].(int64)
		if !didFindItem {
			dl.pool = append(dl.pool, foundItem)
		}
	}
	var derelictItems []int
	for i, on := range dl.pool {
		if on.Source == "oauth" && !contains(updatedIds, on.Cloud_id) {
			derelictItems = append(derelictItems, i)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(derelictItems)))
	for _, on := range derelictItems {
		dl.pool = append(dl.pool[:on], dl.pool[on+1:]...)
	}
	fmt.Printf("%d download(s) in pool currently\n", len(dl.pool))
	return err
}

var downloadPool Downloads
