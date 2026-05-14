package epay

import (
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
)

func Sign(params url.Values, key string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		v := params.Get(k)
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	first := true
	for _, k := range keys {
		if !first {
			buf.WriteString("&")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(params.Get(k))
		first = false
	}
	buf.WriteString(key)

	h := md5.Sum([]byte(buf.String()))
	return hex.EncodeToString(h[:])
}

func Verify(params url.Values, key string) bool {
	sign := params.Get("sign")
	if sign == "" {
		return false
	}
	expected := Sign(params, key)
	return strings.EqualFold(sign, expected)
}
