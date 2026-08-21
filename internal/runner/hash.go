package runner

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
)

func computeHash[T any](items []T) string {
	reprs := make([]string, 0, len(items))
	for _, item := range items {
		reprs = append(reprs, fmt.Sprintf("%+v", item))
	}
	slices.Sort(reprs)

	data, _ := json.Marshal(reprs)

	h := md5.New()
	h.Write(data)

	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
