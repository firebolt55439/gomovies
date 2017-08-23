package main

import (
	"time"
	"encoding/json"
	"context"

	"github.com/go-kit/kit/log"
)

func loggingMiddleware(logger log.Logger) ServiceMiddleware {
	return func(next MovieService) MovieService {
		return logmw{logger, next}
	}
}

type logmw struct {
	logger log.Logger
	MovieService
}

func (mw logmw) Movies(s map[string]interface{}, ctx context.Context) (output map[string]interface{}, err error) {
	defer func(begin time.Time) {
		inp, ok := json.Marshal(s)
		outp, ok := json.Marshal(output)
		if len(outp) > 300 {
			outp = []byte(string(outp[0:300]) + "...")
		}
		if ok != nil {
			panic("Cannot marshal object for logging!")
		}
		_ = mw.logger.Log(
			"method", "movies",
			"input", inp,
			"output", outp,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())

	output, err = mw.MovieService.Movies(s, ctx)
	return
}

func (mw logmw) Count(s string) (n int) {
	defer func(begin time.Time) {
		_ = mw.logger.Log(
			"method", "count",
			"input", s,
			"n", n,
			"took", time.Since(begin),
		)
	}(time.Now())

	n = mw.MovieService.Count(s)
	return
}