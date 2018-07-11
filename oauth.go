package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"net/http"
	"fmt"
	"time"
)

type OAuthInterface interface {
	GetAccessToken(username, password string) (map[string]interface{}, error)
	TestToken() (bool)
	Query(function string, data map[string]interface{}) (map[string]interface{}, error)
	ApiCall(path string, method string, data map[string]interface{}) (map[string]interface{}, error)
}

type OAuth struct {
	/* Required */
	grant_type string
	client_id string
	access_token_url string
	refresh_token_url string
	api_url string
	username string
	password string

	/* Filled in automatically */
	expiry_time time.Time
	access_token string
	refresh_token string
}

func (oa *OAuth) GetAccessToken(username, password string) (map[string]interface{}, error) {
	/* Encode payload */
	payload := url.Values{
		"type": {"login"},
		"grant_type": {oa.grant_type},
		"client_id" : {oa.client_id},
		"username": {username},
		"password": {password},
	}

    /* Send payload */
    res, ok := netClient.PostForm(
		oa.access_token_url,
		payload,
	)
	defer res.Body.Close()
	if ok != nil {
		fmt.Println("Error getting OAuth token:", ok)
		return nil, errors.New("Could not get OAuth token")
	}

	/* Return parsed JSON */
	fmt.Println("Status code:", res.StatusCode)
	var got map[string]interface{}
	json.NewDecoder(res.Body).Decode(&got)
	fmt.Println("Got:", got)
	if _, ok := got["error"]; ok {
		return nil, errors.New(got["error_description"].(string))
	}
	oa.username = username
	oa.password = password
	oa.access_token = got["access_token"].(string)
	oa.refresh_token = got["refresh_token"].(string)
	oa.expiry_time = time.Now().Local().Add(time.Second * time.Duration(got["expires_in"].(float64)))
	return got, nil
}

func (oa *OAuth) TestToken() (bool) {
	/* Skip testing if token not close to expiration */
	if !oa.expiry_time.IsZero() && (oa.expiry_time.Sub(time.Now().Local()) >= (time.Duration(2) * time.Minute)) {
		return true
	}

	/* Test token with API call */
	//fmt.Println("Testing token")
	res, ok := netClient.PostForm(
		oa.api_url,
		url.Values{
			"func": {"test"},
			"access_token": {oa.access_token},
		},
	)
	if ok != nil {
		return false
	}
	defer res.Body.Close()

	/* Parse returned JSON */
	//fmt.Println("Test status code:", res.StatusCode)
	if res.StatusCode == 200 {
		return true
	}
	fmt.Println("Refreshing access token")
	_, err := oa.GetAccessToken(oa.username, oa.password)
	return err == nil
}

func (oa *OAuth) Query(function string, data map[string]interface{}) (map[string]interface{}, error) {
	/* Test token and refresh if needed */
	if !oa.TestToken() {
		return nil, errors.New("Unable to procure valid token")
	}

	/* Generate payload */
	payload := url.Values{
		"func": {function},
		"access_token": {oa.access_token},
	}
	for k, v := range data {
		payload.Set(k, v.(string))
	}

    /* Return parsed JSON */
    res, ok := netClient.PostForm(
		oa.api_url,
		payload,
	)
	if ok != nil {
		return nil, ok
	}
	defer res.Body.Close()

	var got map[string]interface{}
	json.NewDecoder(res.Body).Decode(&got)
	//fmt.Println("Got:", got, "status code:", res.StatusCode)
	if err, ok := got["error"]; ok {
		return nil, errors.New(err.(string))
	}
	return got, nil
}

var oAuth OAuth

func (oa *OAuth) ApiCall(path string, method string, data map[string]interface{}) (map[string]interface{}, error) {
	/* Test token and refresh if needed */
	if !oa.TestToken() {
		return nil, errors.New("Unable to procure valid token")
	}

	/* Generate payload */
	payload := url.Values{}
	for k, v := range data {
		payload.Set(k, v.(string))
	}
	if method != "POST" {
		payload = nil
	}

	/* Generate request */
	target_url := configuration.RestApiUrl + path
	req, _ := http.NewRequest(method, target_url, nil/*payload*/)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oa.access_token))

    /* Return parsed JSON */
    res, ok := netClient.Do(req)
	if ok != nil {
		return nil, ok
	}
	defer res.Body.Close()

	var got map[string]interface{}
	json.NewDecoder(res.Body).Decode(&got)
	//fmt.Println("Got:", got, "status code:", res.StatusCode)
	if err, ok := got["error"]; ok {
		return nil, errors.New(err.(string))
	}
	return got, nil
}
