package goMicroServiceUtils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

/*
=================================================================================
JSON Structures
=================================================================================

=================================================================================
*/

// Tools is the type for this package. Create a variable of this type, and you have access
// to all the exported methods with the receiver type *Tools.
type Tools struct {
	MaxJSONSize        int      // maximum size of JSON file we'll process
	MaxXMLSize         int      // maximum size of XML file we'll process
	MaxFileSize        int      // maximum size of uploaded files in bytes
	AllowedFileTypes   []string // allowed file types for upload (e.g. image/jpeg)
	AllowUnknownFields bool     // if set to true, allow unknown fields in JSON
}

// JSONResponse is the type used for sending JSON around.
type JSONResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// XMLResponse is the type used for sending XML around.
type XMLResponse struct {
	Error   bool        `xml:"error"`
	Message string      `xml:"message"`
	Data    interface{} `xml:"data,omitempty"`
}

type BrokerRequestPayload struct {
	Action string      `json:"action"`
	Auth   AuthPayload `json:"auth,omitempty"`
}

type AuthPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Strcture to dynamically populate templates in front-end without using js
type DisplayResponse struct {
	Output   string
	Payload  string
	Received string
}

/*
=================================================================================
JSON Request/Response Utils
=================================================================================

=================================================================================
*/

// ReadJSON tries to read the body of a request and converts it from JSON to a variable. The third parameter, data,
// is expected to be a pointer, so that we can read data into it.
func (t *Tools) ReadJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {

	// Check content-type header; it should be application/json. If it's not specified,
	// try to decode the body anyway.
	if r.Header.Get("Content-Type") != "" {
		contentType := r.Header.Get("Content-Type")
		if strings.ToLower(contentType) != "application/json" {
			return errors.New("the Content-Type header is not application/json")
		}
	}

	// Set a sensible default for the maximum payload size.
	maxBytes := 1024 * 1024 // one megabyte

	// If MaxJSONSize is set, use that value instead of default.
	if t.MaxJSONSize != 0 {
		maxBytes = t.MaxJSONSize
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)

	// Should we allow unknown fields?
	if !t.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}

	// Attempt to decode the data, and figure out what the error is, if any, to send back a human-readable
	// response.
	err := dec.Decode(data)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			return fmt.Errorf("body contains incorrect JSON type for field %q at offset %d", unmarshalTypeError.Field, unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		case errors.As(err, &invalidUnmarshalError):
			return fmt.Errorf("error unmarshalling json: %s", err.Error())

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}

	return nil
}

// WriteJSON takes a response status code and arbitrary data and writes a JSON response to the client.
func (t *Tools) WriteJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// If we have a value as the last parameter in the function call, then we are setting a custom header.
	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	// Set the content type and send response.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(out)

	return nil
}

// ErrorJSON takes an error, and optionally a response status code, and generates and sends
// a JSON error response.
func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	// If a custom response code is specified, use that instead of bad request.
	if len(status) > 0 {
		statusCode = status[0]
	}

	// Build the JSON payload.
	var payload JSONResponse
	payload.Error = true
	payload.Message = err.Error()

	return t.WriteJSON(w, statusCode, payload)
}
