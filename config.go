package main

type Configuration struct {
	Username string `json:"username"`
	Password string `json:"password"`
	GrantType string `json:"grant_type"`
	ClientId string `json:"client_id"`
	AccessTokenUrl string `json:"access_token_url"`
	RefreshTokenUrl string `json:"refresh_token_url"`
	ApiUrl string `json:"api_url"`
	RestApiUrl string `json:"rest_api_url"`
	
	TraktBaseUrl string `json:"trakt_base_url"`
	TraktClientId string `json:"trakt_client_id"`
	TraktClientSecret string `json:"trakt_client_secret"`
	
	DownloadUriOauth string `json:"download_uri_oauth"`
	DownloadUriOauthParam string `json:"download_uri_oauth_param"`
	
	SourceApiBaseUrl string `json:"source_api_base_url"`
	SourceApiClientId string `json:"source_api_client_id"`
	SourceApiSortKey string `json:"source_api_sort_key"`
	SourceApiResultLimit int `json:"source_api_result_limit"`
	SourceApiHdTitle string `json:"source_api_hd_title"`
	SourceApiSourceKey string `json:"source_api_source_key"`
	SourceApiClientKey string `json:"source_api_client_key"`
}

var configuration = Configuration{}
