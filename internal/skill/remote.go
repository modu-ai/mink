package skill

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"go.uber.org/zap"
)

// LoadRemoteSkill은 HTTP URI에서 SKILL.md를 fetch하여 SkillDefinition을 반환한다.
// REQ-SK-012: remote skill은 _canonical_ 접두사 ID를 가지며, IsRemote == true.
// REQ-SK-022d: 로컬 skill과 동일한 parse-time 보안 정책이 적용된다.
//
// 주의: 인증(AKI/GCS/OAuth)은 Phase 5+에서 구현된다. 현재는 HTTP GET만 수행.
func LoadRemoteSkill(uri string, logger *zap.Logger) (*SkillDefinition, error) {
	// URI 유효성 검사
	parsedURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, fmt.Errorf("잘못된 remote skill URI %q: %w", uri, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("지원되지 않는 URI 스킴 %q (http/https만 지원)", parsedURL.Scheme)
	}

	// HTTP GET
	resp, err := http.Get(uri) //nolint:noctx // TODO Phase 5+에서 context 주입
	if err != nil {
		return nil, fmt.Errorf("remote skill fetch 실패 %q: %w", uri, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote skill fetch 응답 오류 %q: HTTP %d", uri, resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("remote skill 본문 읽기 실패 %q: %w", uri, err)
	}

	// canonical ID 생성: _canonical_ + URI 경로 기반 slug
	slug := deriveRemoteSlug(parsedURL)
	canonicalID := "_canonical_" + slug

	// 파싱 (로컬 skill과 동일한 보안 정책 적용, REQ-SK-022d)
	// AbsolutePath는 URI를 사용 (디렉토리 경로 파생용)
	def, err := ParseSkillFile(uri, content)
	if err != nil {
		return nil, fmt.Errorf("remote skill 파싱 실패 %q: %w", uri, err)
	}

	// remote skill ID를 canonical 형식으로 덮어쓰기
	def.ID = canonicalID
	def.IsRemote = true
	def.AbsolutePath = uri

	logger.Info("remote skill 로드 완료",
		zap.String("skill_id", def.ID),
		zap.String("uri", uri),
	)

	return def, nil
}

// deriveRemoteSlug는 URL에서 human-readable slug를 파생한다.
// 예: http://example.com/skills/my-skill.md → "my-skill"
func deriveRemoteSlug(u *url.URL) string {
	base := path.Base(u.Path)
	// 확장자 제거
	if idx := strings.LastIndex(base, "."); idx != -1 {
		base = base[:idx]
	}
	if base == "" || base == "." || base == "/" {
		base = strings.ReplaceAll(u.Host, ".", "_")
	}
	return base
}
