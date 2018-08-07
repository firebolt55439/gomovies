package main

import (
	"fmt"
	"time"
	"strconv"
	"sort"
	"io/ioutil"
	"io"
	"encoding/json"
	"log"
	"path/filepath"
	"strings"
	"os"
	"os/exec"
	"errors"
	"net/http"
	"bytes"
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
	CloudID string `json:"id"` /* cloud item id, either iCloud Drive or cloud */
	LocalPath string `json:"local_path,omitempty"` /* local path of item */
	Progress float64 `json:"progress,omitempty"` /* progress of current operation */
	ProgressVelocity float64 `json:"progress_velocity,omitempty"` /* progress speed of current operation */
	AvgProgressVelocity float64 `json:"avg_progress_velocity,omitempty"` /* average progress speed of current operation */
	ProgressVelocitySampleCount int `json:"progress_velocity_sample_count,omitempty"` /* number of samples taken */
	TimeStarted int64 `json:"time_started,omitempty"` /* unix timestamp in seconds of start time, if/a */
	Size int64 `json:"size"` /* size of file, -1 if unknown */

	IsDownloadingCloud bool `json:"isDownloadingCloud"` /* true if cloud is downloading item */
	HasDownloadedCloud bool `json:"hasDownloadedCloud"` /* true if cloud has downloaded item */

	IsDownloadingClient bool `json:"isDownloadingClient"` /* true if client is downloading item */
	HasDownloadedClient bool `json:"hasDownloadedClient"` /* true if client has downloaded item */

	IsUploadingClient bool `json:"isUploadingClient"` /* true if client is uploading item */
	HasUploadedClient bool `json:"hasUploadedClient"` /* true if client has uploaded item */

	IsLocalToClient bool `json:"isLocalToClient"` /* true if on local disk */
	Collection string `json:"collection"` /* name of collection item belongs to, if/a */
}

type Downloads struct {
	pool []*DownloadItem
	collections map[string]int
	queue chan interface{}
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

func (dl *Downloads) GetCollections() ([]interface{}) {
	ret := make([]interface{}, 0)
	for k := range dl.collections {
		ret = append(ret, map[string]interface{}{
			"name": k,
			"count": dl.collections[k],
		})
	}
	return ret
}

func (dl *Downloads) ReadiCloudStatus() (map[string]bool, map[string]int, error) {
	/* Dump minified cloud database to temporary file */
	cmd := exec.Command("brctl", "dump", "-i", "-o", "./" + configuration.TemporaryCloudDbFile)
	err := cmd.Run()
	if err != nil {
		return nil, nil, err
	}

	/* Read temporary file */
	data_bytes, err := ioutil.ReadFile(configuration.TemporaryCloudDbFile)
	if err != nil {
		return nil, nil, err
	}

	/* Parse file contents */
	data := string(data_bytes)
	ret := make(map[string]bool)
	size_ret := make(map[string]int)
	data = strings.Split(data, "----------com.apple.CloudDocs")[2]
	data = strings.Split(data, "    ----------------------")[0]
	filename_arr := make([]string, 0)
	for i, on := range strings.Split(data, "reclaimer{evictable:") {
		if strings.Contains(on, "[0;1m") {
			using_on_arr := strings.Split(on, "\n")
			using_on := on
			for _, tmp := range using_on_arr {
				if strings.Contains(tmp, " sz:") {
					using_on = tmp
					break
				}
			}
			filename := strings.Split(using_on, "[0;1m")[1]
			filename = strings.Split(filename, "[0m")[0]
			filename = filename[:len(filename) - 1]

			filename_arr = append(filename_arr, filename)

			size := strings.Split(on, " sz:")[1]
			if strings.Contains(size, "(") {
				size = strings.Split(strings.Split(size, "(")[1], ")")[0]
				size_ret[filename], _ = strconv.Atoi(size)
			}
		}

		if i > 0 {
			is_evictable := on[:3] == "yes"
			filename := filename_arr[i - 1]
			// if is_evictable {
				// fmt.Printf("'%s' --> %t\n", filename, is_evictable)
			// }
			ret[filename] = is_evictable
		}
	}
	return ret, size_ret, nil
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

	/* Read iCloud status from Cloud database */
	isEvictable, sizeMap, _ := dl.ReadiCloudStatus()
	// enc := json.NewEncoder(os.Stdout)
	// enc.Encode(sizeMap)

	/* Walk iCloud drive directory */
	var toAdd []*DownloadItem
	collectionSet := make(map[string]int)
	filepath.Walk(configuration.ICloudDriveFolder, func(path string, f os.FileInfo, err error) error {
		/* Ignore non-video files */
		if !strings.Contains(path, ".mp4") && !strings.Contains(path, ".mkv") {
			return nil
		}

		/* Get file size */
		fi, e := os.Stat(path)
		if e != nil {
			return nil
		}
		size := fi.Size()

		/* Get filename from path and make an educated guess about upload status */
		isUploadingClient := true
		hasUploadedClient := false
		file_components := strings.Split(path, "/")
		filename := file_components[len(file_components) - 1]
		cloud_id_path := path
		is_local := true
		if strings.HasSuffix(filename, ".icloud") {
			is_local = false
			isUploadingClient = false
			hasUploadedClient = true
			filename = filename[1:strings.Index(filename, ".icloud")]
			if _, ok := sizeMap[filename]; ok {
				size = int64(sizeMap[filename])
			} else {
				fmt.Printf("'%s'\n", filename)
				fmt.Println("BAD!")
				size = -1
			}
			file_components[len(file_components) - 1] = filename
			cloud_id_path = strings.Join(file_components, "/")
		} else if is_evictable, ok := isEvictable[filename]; ok {
			if is_evictable {
				hasUploadedClient = true
				isUploadingClient = false
			}
		}

		/* Get current IMDb ID, if possible */
		cloud_id := "icloud_" + cloud_id_path
		var imdb_id string
		if cur_id, ok := cloudToImdb[cloud_id]; ok {
			imdb_id = cur_id
		}

		/* Detect collection name, if it exists */
		collection := ""
		collection_arr := strings.Split(path, configuration.ICloudDriveFolder)
		collection_arr = strings.Split(collection_arr[1], "/")
		for idx, on := range collection_arr {
			if len(on) > 0 && idx != len(collection_arr) - 1 {
				collection = on
				if _, ok := collectionSet[collection]; ok {
					collectionSet[collection]++
				} else {
					collectionSet[collection] = 1
				}
				break
			}
		}

		/* Add item */
		item := &DownloadItem{
			Source: "disk",
			Name: filename,
			CloudID: cloud_id,
			ImdbID: imdb_id,
			Size: size,
			LocalPath: path,
			// Progress: progress,
			IsDownloadingCloud: false,
			HasDownloadedCloud: false,
			IsDownloadingClient: false,
			HasDownloadedClient: true,
			IsUploadingClient: isUploadingClient,
			HasUploadedClient: hasUploadedClient,
			IsLocalToClient: is_local,
			Collection: collection,
		}
		toAdd = append(toAdd, item)
		return nil
	})

	/* Update pool */
	dl.pool = Filter(dl.pool, func(v *DownloadItem) bool {
		return v.Source != "disk"
	})
	dl.pool = append(dl.pool, toAdd...)

	/* Update collections */
	dl.collections = collectionSet
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
			IsLocalToClient: false,
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

func (dl *Downloads) monitorOAuthDownload(cloud_id string, name string) (error) {
	/* Poll folder state in a loop */
	for {
		/* Retrieve main folder */
		res, err := oAuth.ApiCall("folder", "GET", map[string]interface{}{})
		if err != nil {
			return err
		}

		/* Get all folders in main folder. */
		var list []interface{}
		list_tmp, ok := res[configuration.OauthDownloadingPath].([]interface{})
		if !ok {
			return errors.New("Could not retrieve ID's from downloading path")
		}
		list = append(list, list_tmp...)
		list_tmp, ok = res["folders"].([]interface{})
		if !ok {
			return errors.New("Could not retrieve ID's from folders path")
		}
		list = append(list, list_tmp...)

		/* Detect current download progress */
		didFindItem := false
		downloadDone := false
		for _, on_m := range list {
			on := on_m.(map[string]interface{})
			id := fmt.Sprintf("%.0f", on["id"].(float64))
			if id == cloud_id || on["name"].(string) == name {
				didFindItem = true
				if _, ok := on["progress"]; !ok {
					downloadDone = true
					fmt.Println("Cloud is done downloading!")

					/* Retrieve folder listing */
					res, err := oAuth.ApiCall("folder/" + id, "GET", nil)
					if err != nil {
						fmt.Println(err)
						return err
					}

					/* Find the video file */
					var largestSize float64 = -1
					var largestItem map[string]interface{}
					for _, item_i := range res["files"].([]interface{}) {
						item := item_i.(map[string]interface{})
						hasPlayVideo := false
						if _, ok := item["play_video"]; ok {
							hasPlayVideo = true
						}
						hasVideoProgress := false
						if _, ok := item["video_progress"]; ok {
							hasVideoProgress = true
						}
						if !hasPlayVideo && !hasVideoProgress {
							continue
						}
						size := item["size"].(float64)
						if size > largestSize {
							largestItem = item
							largestSize = size
						}
					}
					// b_2, _ := json.MarshalIndent(largestItem, "", "	")
					// fmt.Println(string(b_2))

					/* Retrieve file download URL */
					outp, err := oAuth.Query("fetch_file", map[string]interface{}{
						"folder_file_id": fmt.Sprintf("%.0f", largestItem["folder_file_id"].(float64)),
					})
					// b, _ := json.MarshalIndent(outp, "", "	")
					// fmt.Println(string(b))

					/* Start background download */
					err = dl.StartBackgroundDownload(outp["url"].(string), cloud_id, outp["name"].(string))
					if err != nil {
						fmt.Println(err)
						return err
					}
				}
				break
			}
		}
		if !didFindItem || downloadDone {
			/* If item not found or done downloading, then stop looping */
			break
		}

		/* Sleep before looping again */
		time.Sleep(2500 * time.Millisecond)
	}
	return nil
}

func (dl *Downloads) RegisterOAuthDownloadStart(imdb_id string, cloud_id string, hash_id string, name string) (error) {
	/* Prepend download to beginning of pool so it appears first */
	var err error = nil
	dl.pool = append([]*DownloadItem{&DownloadItem{
		ImdbID: imdb_id,
		Source: "oauth",
		CloudID: cloud_id,
		Name: name,
		Size: -1,
		TimeStarted: time.Now().Unix(),
		IsDownloadingCloud: true,
		HasDownloadedCloud: false,
		IsDownloadingClient: false,
		HasDownloadedClient: false,
		IsUploadingClient: false,
		HasUploadedClient: false,
		IsLocalToClient: false,
	}}, dl.pool...)
	dl.SaveToDisk()

	/* Monitor download progress in background */
	go dl.monitorOAuthDownload(cloud_id, name)
	return err
}

func (dl *Downloads) RegisterOAuthDownloadQueued(imdb_id string, payload map[string]interface{}) (error) {
	dl.queue <- map[string]interface{}{
		"imdb_id": imdb_id,
		"payload": payload,
	}
	return nil
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
		id := fmt.Sprintf("%.0f", on["id"].(float64))
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
			IsLocalToClient: false,
		}
		didFindItem := false
		for _, tmp := range dl.pool {
			if tmp.Source == "oauth" && (tmp.CloudID == id || tmp.Name == on["name"].(string)) {
				foundItem = tmp
				didFindItem = true
				break
			}
		}
		foundItem.CloudID = id
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

func makeTimestamp() int64 {
    return time.Now().UnixNano() / (int64(time.Millisecond)/int64(time.Nanosecond))
}

const (
	DOWNLOAD_UPDATE_MILLISECONDS = 250
)

func (dl *Downloads) monitorDownloadProgress(done chan int64, path string, foundItem *DownloadItem) {
	/* Monitor download progress */
	total := foundItem.Size
	var stop bool = false
	var lastTimestamp int64 = -1
	var lastSize int64 = -1
	for {
		select {
			case <-done:
				stop = true
			default:
				file, err := os.Open(path)
				if err != nil {
					fmt.Println(err)
					return
				}

				fi, err := file.Stat()
				if err != nil {
					fmt.Println(err)
					return
				}

				size := fi.Size()

				if size == 0 {
					size = 1
				}

				percent := float64(size) / float64(total) * 100.0
				foundItem.Progress = percent

				if lastTimestamp != -1 {
					currentTimestamp := makeTimestamp()
					size_added := size - lastSize
					time_elapsed := currentTimestamp - lastTimestamp
					foundItem.ProgressVelocity = 1000.0 * float64(size_added) / float64(time_elapsed)
					foundItem.AvgProgressVelocity = (foundItem.AvgProgressVelocity * float64(foundItem.ProgressVelocitySampleCount) + foundItem.ProgressVelocity) / float64(foundItem.ProgressVelocitySampleCount + 1)
					foundItem.ProgressVelocitySampleCount++
				} else {
					foundItem.ProgressVelocitySampleCount = 0
					foundItem.AvgProgressVelocity = 0
				}

				lastSize = size
				lastTimestamp = makeTimestamp()
		}

		if stop {
			break
		}

		time.Sleep(DOWNLOAD_UPDATE_MILLISECONDS * time.Millisecond)
	}
}

func (dl *Downloads) downloadHelper(url string, filename string, foundItem *DownloadItem) {
	/* Set up download destination */
	dest_path := fmt.Sprintf("%s/%s", configuration.TemporaryDownloadFolder, filename)
	out, err := os.Create(dest_path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer out.Close()

	/* Start monitoring download progress */
	done := make(chan int64)
	go dl.monitorDownloadProgress(done, dest_path, foundItem)

	/* Send GET request */
	fmt.Printf("Starting download at '%s'...\n", url)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	/* Start piping output to disk */
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	/* When done, stop monitoring */
	fmt.Printf("Done with download (%d bytes transferred)\n", n)
	done <- n

	/* Update bool flags */
	foundItem.IsDownloadingClient = false
	foundItem.HasDownloadedClient = false // since a separate entry will crop up
	foundItem.Progress = 102.0

	/* Move file to iCloud drive folder */
	final_path := fmt.Sprintf("%s/%s", configuration.ICloudDriveFolder, filename)
	err = os.Rename(dest_path, final_path)
	if err != nil {
		fmt.Println(err)
		return
	}
	new_cloud_id := "icloud_" + final_path
	dl.pool = append([]*DownloadItem{&DownloadItem{
		Source: "disk",
		Name: filename,
		CloudID: new_cloud_id,
		ImdbID: foundItem.ImdbID,
		Size: foundItem.Size,
		LocalPath: final_path,
		IsDownloadingCloud: false,
		HasDownloadedCloud: false,
		IsDownloadingClient: false,
		HasDownloadedClient: true,
		IsUploadingClient: true,
		HasUploadedClient: false,
		IsLocalToClient: true,
	}}, dl.pool...)
	dl.SaveToDisk()

	/* Delete from cloud */
	_, err = oAuth.Query("delete", map[string]interface{}{
		"delete_arr": "[{\"type\": \"folder\", \"id\": \"" + foundItem.CloudID + "\"}]",
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	/* Intelligently rename just-downloaded file */
	movieWorker := movieData{}
	resolved, err := movieWorker.ResolveImdb(foundItem.ImdbID)
	if err == nil {
		new_title, err := dl.IntelligentRenameItem(new_cloud_id, resolved["title"].(string))
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("Intelligently renamed to '%s'\n", new_title)
		}
	} else {
		fmt.Println(err)
	}

	/* Retrieve main folder */
	res, err := oAuth.ApiCall("folder", "GET", map[string]interface{}{})
	if err != nil {
		fmt.Println(err)
		return
	}

	/* Get all folders in main folder. */
	var list []interface{}
	list_tmp, ok := res[configuration.OauthDownloadingPath].([]interface{})
	if !ok {
		fmt.Println("Could not retrieve ID's from downloading path")
		return
	}
	list = append(list, list_tmp...)
	list_tmp, ok = res["folders"].([]interface{})
	if !ok {
		fmt.Println("Could not retrieve ID's from folders path")
		return
	}
	list = append(list, list_tmp...)

	/* Refresh current download states */
	downloadPool.RefreshDownloadStates(list)
	downloadPool.RefreshDiskDownloads()

	/* Pop next item off queue if it exists */
	counter := 0
	for {
		var q map[string]interface{}
		select {
			case tmp, ok := <-dl.queue:
				if !ok {
					fmt.Println("Channel closed!")
					return
				} else {
					q = tmp.(map[string]interface{})
				}
			default:
				return
		}
		outp, err := oAuth.Query(configuration.DownloadUriOauth, q["payload"].(map[string]interface{}))
		if err != nil {
			fmt.Println(err)
			return
		}

		/* If not enough space, push item to back of queue */
		if result, ok := outp["result"].(string); ok &&
		(strings.Contains(result, "not_enough_space") || strings.Contains(result, "queue_full")) {
			dl.queue <- q
			fmt.Println("Not enough space --> pushing item to back of queue")
			counter++
			if counter > 3 {
				counter = 0

				/* Retrieve main folder */
				res, err := oAuth.ApiCall("folder", "GET", map[string]interface{}{})
				if err != nil {
					fmt.Println(err)
					return
				}

				/* Get all ID's from main folder. */
				var list []interface{}
				list_tmp, ok := res[configuration.OauthDownloadingPath].([]interface{})
				if !ok {
					fmt.Println("Could not retrieve ID's from downloading path")
					return
				}
				list = append(list, list_tmp...)
				list_tmp, ok = res["folders"].([]interface{})
				if !ok {
					fmt.Println("Could not retrieve ID's from folders path")
					return
				}
				list = append(list, list_tmp...)

				/* Clear out main folder */
				fmt.Println("folders:", list)
				for _, item := range list {
					conv_item, ok := item.(map[string]interface{})
					if !ok {
						fmt.Println("Could not convert folder item")
						return
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
						fmt.Println(err)
						return
					}
				}
			}
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		/* Otherwise, go ahead and process item */
		dl.RegisterOAuthDownloadStart(
			q["imdb_id"].(string),
			fmt.Sprintf("%.0f", outp[configuration.CloudItemIdKey].(float64)),
			outp[configuration.CloudHashIdKey].(string),
			outp["title"].(string),
		)
		break
	}
}

func (dl *Downloads) StartBackgroundDownload(url, cloud_id, filename string) (error) {
	/* Find item in pool */
	var foundItem *DownloadItem = nil
	for _, on := range dl.pool {
		if on.CloudID == cloud_id {
			foundItem = on
			break
		}
	}
	if foundItem == nil {
		return errors.New("Could not find item with matching cloud_id")
	}
	foundItem.IsDownloadingClient = true
	foundItem.HasDownloadedClient = false
	foundItem.Progress = 0.0

	/* Sanitize URL */
	url = strings.Replace(url, "+", "%20", -1)

	/* Perform an HTTP HEAD request to detect file length */
	headResp, err := http.Head(url)
	if err != nil {
		return err
	}
	defer headResp.Body.Close()

	size_int, _ := strconv.Atoi(headResp.Header.Get("Content-Length"))
	foundItem.Size = int64(size_int)

	/* Start download helper in background */
	go dl.downloadHelper(url, filename, foundItem)

	return nil
}

func (dl *Downloads) EvictLocalItem(cloud_id string) (error) {
	/* Find item in pool */
	var foundItem *DownloadItem = nil
	for _, on := range dl.pool {
		if on.CloudID == cloud_id {
			foundItem = on
			break
		}
	}
	if foundItem == nil {
		return errors.New("Could not find item with matching cloud_id")
	}

	/* Verify item flags */
	if !foundItem.HasDownloadedClient {
		return errors.New("Item is not downloaded on disk")
	}
	if !foundItem.HasUploadedClient {
		return errors.New("Item not uploaded to iCloud yet")
	}
	if !foundItem.IsLocalToClient {
		return errors.New("Item already in iCloud")
	}
	if foundItem.Source == "oauth" {
		return errors.New("Item not taken from disk")
	}
	if len(foundItem.LocalPath) == 0 {
		return errors.New("No path found in item")
	}

	/* Execute eviction and return if any error */
	cmd := exec.Command("brctl", "evict", foundItem.LocalPath)
	err := cmd.Run()
	return err
}

func (dl *Downloads) AddToCollection(cloud_id string, collection_id string) (error) {
	/* Find item in pool */
	var foundItem *DownloadItem = nil
	for _, on := range dl.pool {
		if on.CloudID == cloud_id {
			foundItem = on
			break
		}
	}
	if foundItem == nil {
		return errors.New("Could not find item with matching cloud_id")
	}

	/* Verify item flags */
	if !foundItem.HasUploadedClient {
		return errors.New("Item not uploaded to iCloud")
	}
	if foundItem.Source == "oauth" {
		return errors.New("Item not taken from disk")
	}
	if len(foundItem.LocalPath) == 0 {
		return errors.New("No path found in item")
	}

	/* Execute collection move and return result */
	collection_path := filepath.Join(
		configuration.ICloudDriveFolder,
		collection_id,
	)
	os.MkdirAll(collection_path, os.ModePerm)
	final_path_fake := filepath.Join(
		collection_path,
		foundItem.Name,
	)
	split_arr := strings.Split(foundItem.LocalPath, "/")
	final_path := filepath.Join(
		collection_path,
		split_arr[len(split_arr) - 1],
	)
	err := os.Rename(foundItem.LocalPath, final_path)
	if err != nil {
		return err
	}

	/* Append filler entry for new path to ensure IMDb ID association is not lost */
	dl.pool = append(dl.pool, &DownloadItem{
		Source: "disk",
		CloudID: "icloud_" + final_path_fake,
		ImdbID: foundItem.ImdbID,
		LocalPath: final_path,
	})
	return nil
}

func (dl *Downloads) GetiCloudStreamUrl(cloud_id string) (string, error) {
	/* Find item in pool */
	var foundItem *DownloadItem = nil
	for _, on := range dl.pool {
		if on.CloudID == cloud_id {
			foundItem = on
			break
		}
	}
	if foundItem == nil {
		return "", errors.New("Could not find item with matching cloud_id")
	}

	/* Verify item flags */
	if !foundItem.HasUploadedClient {
		return "", errors.New("Item not uploaded to iCloud")
	}
	if foundItem.Source == "oauth" {
		return "", errors.New("Item not taken from disk")
	}
	if len(foundItem.LocalPath) == 0 {
		return "", errors.New("No path found in item")
	}

	/* Execute eviction and return if any error */
	var out bytes.Buffer
	cmd := exec.Command("swift", "stream_link.swift", foundItem.LocalPath)
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

func (dl *Downloads) IntelligentRenameItem(cloud_id, title string) (string, error) {
	/* Find item in pool */
	var foundItem *DownloadItem = nil
	for _, on := range dl.pool {
		if on.CloudID == cloud_id {
			foundItem = on
			break
		}
	}
	if foundItem == nil {
		return "", errors.New("Could not find item with matching cloud_id")
	}

	/* Verify item flags */
	if !foundItem.IsLocalToClient {
		return "", errors.New("Item already in iCloud")
	}
	if !foundItem.HasDownloadedClient {
		return "", errors.New("Item not downloaded to disk")
	}
	if foundItem.Source == "oauth" {
		return "", errors.New("Item not taken from disk")
	}
	if len(foundItem.LocalPath) == 0 {
		return "", errors.New("No path found in item")
	}
	if len(foundItem.ImdbID) == 0 {
		return "", errors.New("Item is unassociated")
	}

	/* Generate new name */
	name_components := strings.Split(foundItem.Name, ".")
	file_extension := name_components[len(name_components) - 1]
	new_name := title
	new_name = strings.Replace(new_name, ":", "", -1)
	new_name = strings.Replace(new_name, "/", "", -1)
	new_name = strings.Replace(new_name, "(", "", -1)
	new_name = strings.Replace(new_name, ")", "", -1)
	new_name = strings.Replace(new_name, " - ", " ", -1)
	for strings.Contains(new_name, "  ") {
		new_name = strings.Replace(new_name, "  ", " ", -1)
	}
	new_name = strings.Replace(new_name, " ", ".", -1)
	new_name += "." + file_extension

	/* Execute rename and return error, if any */
	path_components := strings.Split(foundItem.LocalPath, "/")
	path_components[len(path_components) - 1] = new_name
	final_path := strings.Join(path_components, "/")
	err := os.Rename(foundItem.LocalPath, final_path)
	if err != nil {
		return "", err
	}

	/* Append filler entry for new path to ensure IMDb ID association is not lost */
	dl.pool = append(dl.pool, &DownloadItem{
		Source: "disk",
		CloudID: "icloud_" + final_path,
		ImdbID: foundItem.ImdbID,
		LocalPath: final_path,
	})
	return new_name, err
}

var downloadPool Downloads
