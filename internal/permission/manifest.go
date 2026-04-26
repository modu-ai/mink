package permission

// Capability는 requires: 스키마의 4-카테고리 권한 유형을 나타낸다.
// REQ-PE-002
type Capability int

const (
	// CapNet은 네트워크 host 접근 권한이다 (host literal 또는 host glob).
	CapNet Capability = iota
	// CapFSRead는 파일시스템 읽기 권한이다 (path glob, FS-ACCESS-001 표기 호환).
	CapFSRead
	// CapFSWrite는 파일시스템 쓰기 권한이다 (path glob).
	CapFSWrite
	// CapExec는 프로세스 실행 권한이다 (binary basename).
	CapExec
)

// String은 Capability의 문자열 표현을 반환한다.
// grants.json 직렬화 및 CLI 출력에 사용된다.
func (c Capability) String() string {
	switch c {
	case CapNet:
		return "net"
	case CapFSRead:
		return "fs_read"
	case CapFSWrite:
		return "fs_write"
	case CapExec:
		return "exec"
	default:
		return "unknown"
	}
}

// CapabilityFromString은 문자열을 Capability로 변환한다.
// 알 수 없는 문자열에는 -1을 반환한다.
func CapabilityFromString(s string) (Capability, bool) {
	switch s {
	case "net":
		return CapNet, true
	case "fs_read":
		return CapFSRead, true
	case "fs_write":
		return CapFSWrite, true
	case "exec":
		return CapExec, true
	default:
		return -1, false
	}
}

// Manifest는 frontmatter `requires:` 파싱 결과다.
// RequiresParser.Parse가 반환한다.
// REQ-PE-002
type Manifest struct {
	// NetHosts는 net 카테고리의 host literal 또는 host glob 목록이다.
	NetHosts []string
	// FSReadPaths는 fs_read 카테고리의 path glob 목록이다.
	FSReadPaths []string
	// FSWritePaths는 fs_write 카테고리의 path glob 목록이다.
	FSWritePaths []string
	// ExecBinaries는 exec 카테고리의 binary basename 목록이다.
	ExecBinaries []string
}

// Declares는 주어진 capability/scope 조합이 manifest에 선언되어 있는지 확인한다.
// 정확히 일치하는 scope 토큰이 있어야 한다 (glob 매칭은 FS-ACCESS-001 위임).
func (m *Manifest) Declares(cap Capability, scope string) bool {
	var list []string
	switch cap {
	case CapNet:
		list = m.NetHosts
	case CapFSRead:
		list = m.FSReadPaths
	case CapFSWrite:
		list = m.FSWritePaths
	case CapExec:
		list = m.ExecBinaries
	}
	for _, s := range list {
		if s == scope {
			return true
		}
	}
	return false
}

// RequiresParser는 frontmatter raw map → Manifest 변환을 수행한다.
// 알 수 없는 카테고리는 errs에 누적되고, 4 카테고리는 가능한 만큼 채운다.
// REQ-PE-002, REQ-PE-010, REQ-PE-018
type RequiresParser struct{}

// Parse는 frontmatter의 requires 키 값(map[string]any)을 받아 Manifest로 변환한다.
//
// 알 수 없는 카테고리: ErrUnknownCapability 누적 (parse 중단 없음).
// 스칼라 값: ErrInvalidScopeShape 누적, 해당 카테고리는 nil.
// 중첩 requires: ErrInvalidScopeShape{Nested:true} 누적, 모든 필드 nil.
//
// REQ-PE-002, REQ-PE-010, REQ-PE-018, AC-PE-001
func (p *RequiresParser) Parse(raw map[string]any) (Manifest, []error) {
	var m Manifest
	var errs []error

	// REQ-PE-018: 중첩 requires: 구조 검사
	// raw 값 중에 "requires" 키가 있으면 중첩 구조로 판정한다.
	if _, hasNested := raw["requires"]; hasNested {
		errs = append(errs, ErrInvalidScopeShape{Nested: true})
		return Manifest{}, errs
	}

	for key, val := range raw {
		cap, ok := CapabilityFromString(key)
		if !ok {
			errs = append(errs, ErrUnknownCapability{Key: key})
			continue
		}

		// REQ-PE-010: 스칼라 값 거부 (배열 아닌 경우)
		strSlice, parseErr := toStringSlice(key, val)
		if parseErr != nil {
			errs = append(errs, parseErr)
			// 스칼라 coercion 금지: 해당 카테고리는 nil로 유지
			continue
		}

		switch cap {
		case CapNet:
			m.NetHosts = strSlice
		case CapFSRead:
			m.FSReadPaths = strSlice
		case CapFSWrite:
			m.FSWritePaths = strSlice
		case CapExec:
			m.ExecBinaries = strSlice
		}
	}

	return m, errs
}

// toStringSlice는 any 값을 []string으로 변환한다.
// 스칼라이면 ErrInvalidScopeShape를 반환한다.
func toStringSlice(category string, val any) ([]string, error) {
	switch v := val.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, ErrInvalidScopeShape{Category: category, Value: val}
			}
			result = append(result, s)
		}
		return result, nil
	case []string:
		return v, nil
	default:
		// 스칼라 또는 알 수 없는 타입 — coercion 금지
		return nil, ErrInvalidScopeShape{Category: category, Value: val}
	}
}
