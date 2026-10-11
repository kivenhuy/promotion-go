package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type drawRequest struct {
	UserID   int    `json:"user_id"`
	Campaign string `json:"campaign"`
}

func (r drawRequest) validate() error {
	if r.UserID <= 0 {
		return errors.New("user_id phải lớn hơn 0")
	}
	if r.Campaign == "" {
		return errors.New("campaign không được rỗng")
	}
	return nil
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	const MaxBytes = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err == nil {
		return nil
	}

	var typeErr *json.UnmarshalTypeError
	var maxErr *http.MaxBytesError

	switch {
	case errors.Is(err, io.EOF):
		return errors.New("body rỗng")

	case errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New("JSON không hợp lệ")

	case errors.As(err, &typeErr):
		return fmt.Errorf("field %q sai kiểu dữ liệu", typeErr.Field)

	case errors.As(err, &maxErr):
		return fmt.Errorf("body quá lớn, tối đa %d byte", MaxBytes)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		name := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Errorf("field %s không tồn tại", name)
	default:
		return err
	}
}
