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

		/* Get filename from path and make an educated guess about upload status */
		isUploadingClient := true
		hasUploadedClient := false
		file_components := strings.Split(path, "/")
		filename := file_components[len(file_components) - 1]
		cloud_id_path := path
		if strings.HasSuffix(filename, ".icloud") {
			isUploadingClient = false
			hasUploadedClient = true
			filename = filename[1:strings.Index(filename, ".icloud")]
			size = -1
			file_components[len(file_components) - 1] = filename
			cloud_id_path = strings.Join(file_components, "/")
		}

		/* Get current IMDb ID, if possible */
		cloud_id := "icloud_" + cloud_id_path
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
			LocalPath: path,
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
	}}, dl.pool...)
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

func (dl *Downloads) monitorDownloadProgress(done chan int64, path string, foundItem *DownloadItem) {
	/* Monitor download progress */
	total := foundItem.Size
	var stop bool = false
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
		}

		if stop {
			break
		}

		time.Sleep(125 * time.Millisecond)
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
	fmt.Printf("Done with download (%ld bytes transferred)\n", n)
	done <- n

	/* Update bool flags */
	foundItem.IsDownloadingClient = false
	foundItem.HasDownloadedClient = false // since a separate entry will crop up
	foundItem.Progress = 102.0

	/* Move file to iCloud drive folder */
	final_path := fmt.Sprintf("%s/%s", configuration.ICloudDriveFolder, filename)
	os.Rename(dest_path, final_path)
	dl.pool = append([]*DownloadItem{&DownloadItem{
		Source: "disk",
		Name: filename,
		CloudID: "icloud_" + final_path,
		ImdbID: foundItem.ImdbID,
		Size: foundItem.Size,
		// Progress: progress,
		IsDownloadingCloud: false,
		HasDownloadedCloud: false,
		IsDownloadingClient: false,
		HasDownloadedClient: true,
		IsUploadingClient: true,
		HasUploadedClient: false,
	}}, dl.pool...)
	dl.SaveToDisk()
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
	if foundItem.HasUploadedClient {
		return errors.New("Item already uploaded to iCloud")
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
	if foundItem.HasUploadedClient {
		return "", errors.New("Item already uploaded to iCloud")
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
	new_name = strings.Replace(new_name, " ", ".", -1)
	new_name = strings.Replace(new_name, ":", "", -1)
	new_name = strings.Replace(new_name, "/", "", -1)
	new_name = strings.Replace(new_name, "(", "", -1)
	new_name = strings.Replace(new_name, ")", "", -1)
	new_name = strings.Replace(new_name, "..", "", -1)
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
