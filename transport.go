package main

import (
	//"fmt"
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	//httptransport "github.com/go-kit/kit/transport/http"
)

/* Request Types */
type moviesRequest struct {
	S map[string]interface{} `json:"q"`
}

type countRequest struct {
	S string `json:"s"`
}

/* Response Types */
type moviesResponse struct {
	V map[string]interface{} `json:"v"`
	Err string `json:"err,omitempty"`
}

type countResponse struct {
	V int `json:"v"`
}

/* Specific helper functions to decode responses */
func decodeMoviesResponse(_ context.Context, r *http.Response) (interface{}, error) {
	var response moviesResponse
	if err := json.NewDecoder(r.Body).Decode(&response); err != nil {
		return nil, err
	}
	return response, nil
}

/* Specific helper functions to create API endpoints */
func makeMoviesEndpoint(svc MovieService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(moviesRequest)
		v, err := svc.Movies(req.S, ctx)
		if err != nil {
			return moviesResponse{v, err.Error()}, nil
		}
		return moviesResponse{v, ""}, nil
	}
}

func makeCountEndpoint(svc MovieService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (interface{}, error) {
		req := request.(countRequest)
		v := svc.Count(req.S)
		return countResponse{v}, nil
	}
}

/* Generic helper functions to decode requests and responses */
func decodeMoviesRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	var request moviesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func decodeCountRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request countRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

/* Generic helper functions to encode requests and responses */
func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}

func encodeRequest(_ context.Context, r *http.Request, request interface{}) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(request); err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(&buf)
	return nil
}