package main

type SourceConfig map[string]interface{}

type Configuration struct {
	Username string `json:"username"`
	Password string `json:"password"`
	GrantType string `json:"grant_type"`
	ClientId string `json:"client_id"`
	AccessTokenUrl string `json:"access_token_url"`
	RefreshTokenUrl string `json:"refresh_token_url"`
	ApiUrl string `json:"api_url"`
	RestApiUrl string `json:"rest_api_url"`
	
	ClientTimeoutSeconds int `json:"client_timeout_seconds"`
	ClientMaxRetries int `json:"client_max_retries"`
	ClientConcurrency int `json:"client_concurrency"`

	ICloudDriveFolder string `json:"icloud_drive_folder"`
	
	TraktBaseUrl string `json:"trakt_base_url"`
	TraktClientId string `json:"trakt_client_id"`
	TraktClientSecret string `json:"trakt_client_secret"`
	TraktAccessToken string `json:"trakt_access_token"`
	TraktRefreshToken string `json:"trakt_refresh_token"`
	
	DownloadUriOauth string `json:"download_uri_oauth"`
	DownloadUriOauthParam string `json:"download_uri_oauth_param"`
	OauthDownloadingPath string `json:"oauth_downloading_path"`
	CloudItemIdKey string `json:"cloud_item_id_key"`
	CloudHashIdKey string `json:"cloud_hash_id_key"`
	
	TitleQualityHDKeywords []string `mapstructure:"hd_titles"`

	OmdbApiKeys []string `json:"omdbapi_keys"`
	
	Sources []SourceConfig `json:"sources"`
}

var configuration = Configuration{}
