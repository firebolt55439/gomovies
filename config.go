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
}

var configuration = Configuration{}
