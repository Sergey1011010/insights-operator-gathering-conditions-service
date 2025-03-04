package service

import (
	"encoding/json"
	"errors"
	"net/http"

	merrors "github.com/redhatinsights/insights-operator-conditional-gathering/internal/errors"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type GatheringRulesResponse struct {
	Version string      `json:"version"`
	Rules   interface{} `json:"rules"`
}

func gatheringRulesEndpoint(svc Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := svc.Rules()
		if err != nil {
			renderErrorResponse(w, "internal error", err)
			return
		}

		renderResponse(w, &GatheringRulesResponse{
			Version: "1.0",
			Rules:   rules.Items,
		}, http.StatusOK)
	}
}

func renderResponse(w http.ResponseWriter, resp interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")

	content, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(code)

	if _, err = w.Write(content); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func renderErrorResponse(w http.ResponseWriter, msg string, err error) {
	resp := ErrorResponse{Error: msg}
	code := http.StatusInternalServerError

	var ierr *merrors.Error
	if !errors.As(err, &ierr) {
		resp.Error = "internal error"
	} else {
		switch ierr.Code() {
		case merrors.ErrorCodeNotFound:
			code = http.StatusNotFound
		case merrors.ErrorCodeInvalidArgument:
			code = http.StatusBadRequest
		case merrors.ErrorCodeUnknown:
			code = http.StatusInternalServerError
		}
	}

	renderResponse(w, resp, code)
}
