package main

import (
	"fmt"
	"time"
	"strconv"
	"sort"
	"io/ioutil"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"os"
)

const (
	DISK_SAVE_FILENAME = "downloads.json"
)

type DiskDownloadItem struct {
	ImdbID string `json:"imdb_id"`
	Filename string `json:"filename,omitempty"` /* filename */
	Source string `json:"source"` /* type of source (e.g. "oauth", "disk", etc.) */
	Size int64 `json:"size"` /* size of item */
	CloudID string `json:"id"` /* cloud item id, either iCloud Drive or cloud, if/a */
}

type DownloadItem struct {
	ImdbID string `json:"imdb_id"`
	Source string `json:"source"` /* "disk", "oauth", etc. */
	Name string `json:"name"` /* name of item */
	CloudID string `json:"id"` /* cloud item id, either iCloud Drive or cloud, if/a */
	Progress float64 `json:"progress,omitempty"` /* progress of current operation */
	TimeStarted int64 `json:"time_started,omitempty"` /* unix timestamp in seconds of start time, if/a */
	Size int64 `json:"size"` /* size of file, -1 if unknown */

	IsDownloadingCloud bool `json:"isDownloadingCloud"` /* true if cloud is downloading item */
	HasDownloadedCloud bool `json:"hasDownloadedCloud"` /* true if cloud has downloaded item */

	IsDownloadingClient bool `json:"isDownloadingClient"` /* true if client is downloading item */
	HasDownloadedClient bool `json:"hasDownloadedClient"` /* true if client has downloaded item */

	IsUploadingClient bool `json:"isUploadingClient"` /* true if client is uploading item */
	HasUploadedClient bool `json:"hasUploadedClient"` /* true if client has uploaded item */
}

type Downloads struct {
	pool []*DownloadItem
}

func Filter(vs []*DownloadItem, f func(*DownloadItem) bool) []*DownloadItem {
    vsf := make([]*DownloadItem, 0)
    for _, v := range vs {
        if f(v) {
            vsf = append(vsf, v)
        }
    }
    return vsf
}

func (dl *Downloads) GetAssociatedDownloads() ([]string) {
	ret := make([]string, 0)
	arr := Filter(dl.pool, func(v *DownloadItem) bool {
		return len(v.ImdbID) > 0
	})
	for _, on := range arr {
		ret = append(ret, on.ImdbID)
	}
	return ret
}

func (dl *Downloads) RefreshDiskDownloads() {
	/* Generate cloud id to imdb id mapping for current disk downloads */
	cloudToImdb := make(map[string]string)
	for _, on := range dl.pool {
		if on.Source != "disk" {
			continue
		}
		cloudToImdb[on.CloudID] = on.ImdbID
	}

	/* Walk iCloud drive directory */
	var toAdd []*DownloadItem
	filepath.Walk(configuration.ICloudDriveFolder, func(path string, f os.FileInfo, err error) error {
		/* Ignore non-video files */
		if !strings.Contains(path, ".mp4") {
			return nil
		}

		/* Get file size */
		fi, e := os.Stat(path)
		if e != nil {
			return nil
		}
		size := fi.Size()

		/* Get filename from path */
		file_components := strings.Split(path, "/")
		filename := file_components[len(file_components) - 1]
		if strings.HasSuffix(filename, ".icloud") {
			filename = filename[1:strings.Index(filename, ".icloud")]
			size = -1
		}

		/* Check if uploading to iCloud or not and retrieve progress */
		// progress := -1.0
		isUploadingClient := false
		hasUploadedClient := false

		/* Get current IMDb ID, if possible */
		cloud_id := "icloud_" + path
		var imdb_id string
		if cur_id, ok := cloudToImdb[cloud_id]; ok {
			imdb_id = cur_id
		}

		/* Add item */
		item := &DownloadItem{
			Source: "disk",
			Name: filename,
			CloudID: cloud_id,
			ImdbID: imdb_id,
			Size: size,
			// Progress: progress,
			IsDownloadingCloud: false,
			HasDownloadedCloud: false,
			IsDownloadingClient: false,
			HasDownloadedClient: true,
			IsUploadingClient: isUploadingClient,
			HasUploadedClient: hasUploadedClient,
		}
		toAdd = append(toAdd, item)
		return nil
	})

	/* Update pool */
	dl.pool = Filter(dl.pool, func(v *DownloadItem) bool {
		return v.Source != "disk"
	})
	dl.pool = append(dl.pool, toAdd...)
}

func (dl *Downloads) ReadFromDisk() {
	/* Read iCloud Drive files from disk */
	dl.RefreshDiskDownloads()

	/* Read saved JSON file */
	content, err := ioutil.ReadFile(DISK_SAVE_FILENAME)
	if err != nil {
		log.Print(err)
		return
	}
	var savedArr []DiskDownloadItem
	json.Unmarshal(content, &savedArr)

	/* Restore OAuth items */
	for _, on := range savedArr {
		if on.Source != "oauth" {
			continue
		}
		dl.pool = append(dl.pool, &DownloadItem{
			ImdbID: on.ImdbID,
			Source: "oauth",
			CloudID: on.CloudID,
			Name: on.Filename,
			Size: on.Size,
			IsDownloadingCloud: false,
			HasDownloadedCloud: true,
			IsDownloadingClient: false,
			HasDownloadedClient: false,
			IsUploadingClient: false,
			HasUploadedClient: false,
		})
	}

	/* Restore iCloud Drive associations */
	for _, on := range savedArr {
		if on.Source != "disk" {
			continue
		}
		for _, at := range dl.pool {
			if at.CloudID == on.CloudID {
				at.ImdbID = on.ImdbID
			}
		}
	}
}

func (dl *Downloads) SaveToDisk() (error) {
	assoc := 0
	unassoc := 0
	var err error = nil
	var arr []DiskDownloadItem
	for _, on := range dl.pool {
		if len(on.ImdbID) == 0 {
			unassoc += 1
		} else {
			assoc += 1
		}
		obj := DiskDownloadItem{
			ImdbID: on.ImdbID,
			Filename: on.Name,
			Source: on.Source,
			Size: on.Size,
			CloudID: on.CloudID,
		}
		arr = append(arr, obj)
	}
	fmt.Printf("%d download(s) currently in pool (%d assoc., %d unassoc.)\n", len(dl.pool), assoc, unassoc)
	pool_json, err := json.Marshal(arr)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(DISK_SAVE_FILENAME, pool_json, 0644)
	return err
}

func (dl *Downloads) RetrieveDownloads() ([]DownloadItem) {
	var pool []DownloadItem
	for _, on := range dl.pool {
		pool = append(pool, *on)
	}
	return pool
}

func (dl *Downloads) AssociateDownloadWithImdb(download_id string, imdb_id string) (bool) {
	for _, on := range dl.pool {
		if on.CloudID == download_id {
			on.ImdbID = imdb_id
			fmt.Println("associated", on)
			dl.SaveToDisk()
			return true
		}
	}
	return false
}

func (dl *Downloads) RegisterOAuthDownloadStart(imdb_id string, cloud_id string, name string) (error) {
	var err error = nil
	dl.pool = append(dl.pool, &DownloadItem{
		ImdbID: imdb_id,
		Source: "oauth",
		CloudID: cloud_id,
		Name: name,
		Progress: -1.0,
		Size: -1,
		TimeStarted: time.Now().Unix(),
		IsDownloadingCloud: true,
		HasDownloadedCloud: false,
		IsDownloadingClient: false,
		HasDownloadedClient: false,
		IsUploadingClient: false,
		HasUploadedClient: false,
	})
	dl.SaveToDisk()
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
			ImdbID: "",
			Source: "oauth",
			Name: on["name"].(string),
			CloudID: id,
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
			if tmp.Source == "oauth" && tmp.CloudID == id {
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
		foundItem.Size = int64(on["size"].(float64))
		if !didFindItem {
			dl.pool = append(dl.pool, foundItem)
		}
	}
	var derelictItems []int
	for i, on := range dl.pool {
		if on.Source == "oauth" && !contains(updatedIds, on.CloudID) {
			derelictItems = append(derelictItems, i)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(derelictItems)))
	for _, on := range derelictItems {
		dl.pool = append(dl.pool[:on], dl.pool[on+1:]...)
	}
	dl.SaveToDisk()
	return err
}

var downloadPool Downloads
