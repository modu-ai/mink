package router

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// makeSignature는 Route에 대한 canonical signature 문자열을 생성한다.
//
// 형식: "model|provider|base_url|mode|command|args_hash_12"
// args_hash_12 = sha256(canonicalJSON(args))의 앞 12 hex 문자.
//
// REQ-ROUTER-014: 시간, credential, 사용자 식별자를 포함하지 않는다.
// REQ-ROUTER-002: 동일 Route 입력에 대해 동일 signature를 반환한다.
func makeSignature(r *Route) string {
	argsHash := hashArgs(r.Args)
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		r.Model, r.Provider, r.BaseURL, r.Mode, r.Command, argsHash)
}

// hashArgs는 Args 맵을 canonical JSON으로 직렬화한 후 sha256 해시의 앞 12자를 반환한다.
// 키 정렬로 결정적 출력을 보장한다.
func hashArgs(args map[string]any) string {
	canonical := canonicalJSON(args)
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", sum)[:12]
}

// canonicalJSON은 맵을 키 알파벳 순으로 정렬한 JSON 문자열로 직렬화한다.
// 결정적 출력을 위해 키 순서를 고정한다.
func canonicalJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kv, _ := json.Marshal(k)
		vv, _ := json.Marshal(m[k])
		buf.Write(kv)
		buf.WriteByte(':')
		buf.Write(vv)
	}
	buf.WriteByte('}')
	return buf.String()
}
