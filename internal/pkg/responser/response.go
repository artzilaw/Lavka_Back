package responser

import (
	"encoding/json"
	"net/http"
)

const ErrJSONMarshalFailed = "couldn't convert the received data to JSON"

func RespondWithJSON(w http.ResponseWriter, code int, payload any) {
	response, err := json.Marshal(payload)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, ErrJSONMarshalFailed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, map[string]string{"error": message})
}
