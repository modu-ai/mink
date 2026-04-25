package skill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// LoadOption은 LoadSkillsDir의 옵션 함수 타입이다.
type LoadOption func(*loadOptions)

type loadOptions struct {
	logger *zap.Logger
}

// WithLogger는 SkillRegistry에 사용할 로거를 설정한다.
func WithLogger(logger *zap.Logger) LoadOption {
	return func(opts *loadOptions) {
		opts.logger = logger
	}
}

// LoadSkillsDir은 root 디렉토리를 재귀적으로 탐색하여 SKILL.md 파일을 파싱하고
// SkillRegistry를 구성한다.
// REQ-SK-005: partial-success 방식 — 개별 파일 실패는 error slice에 추가하고 로드 계속.
// REQ-SK-015: symlink escape 방지 (ErrSymlinkEscape).
// REQ-SK-004: 중복 ID는 ErrDuplicateSkillID로 error slice에 추가, 첫 번째 항목 보존.
//
// @MX:ANCHOR: [AUTO] LoadSkillsDir — 모든 skill 진입점, hot-reload + 부트스트랩 모두 호출
// @MX:REASON: 모든 skill의 초기 로드는 이 함수를 통과한다. fan_in >= 3 (main, test, plugin)
// @MX:SPEC: REQ-SK-002, REQ-SK-005
func LoadSkillsDir(root string, opts ...LoadOption) (*SkillRegistry, []error) {
	options := &loadOptions{
		logger: zap.NewNop(),
	}
	for _, opt := range opts {
		opt(options)
	}

	// root를 절대 경로로 정규화
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return NewSkillRegistry(options.logger), []error{
			fmt.Errorf("root 경로 절대화 실패: %w", err),
		}
	}

	reg := NewSkillRegistry(options.logger)
	var errs []error
	firstRegistered := make(map[string]string) // id → 첫 번째 파일 경로

	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, fmt.Errorf("walk 오류 %s: %w", path, err))
			return nil // 계속 진행
		}

		// SKILL.md 파일만 처리
		if d.IsDir() || d.Name() != "SKILL.md" {
			return nil
		}

		// symlink escape 방지 (REQ-SK-015)
		if d.Type()&fs.ModeSymlink != 0 || isSymlink(path) {
			realPath, evalErr := filepath.EvalSymlinks(path)
			if evalErr != nil {
				errs = append(errs, ErrSymlinkEscape{Path: path})
				options.logger.Warn("symlink 평가 실패 — 건너뜀",
					zap.String("path", path),
					zap.Error(evalErr),
				)
				return nil
			}

			// 실제 경로가 root 외부인지 확인
			if !strings.HasPrefix(realPath, absRoot) {
				errs = append(errs, ErrSymlinkEscape{Path: path})
				options.logger.Warn("symlink escape 감지 — 건너뜀",
					zap.String("path", path),
					zap.String("resolved", realPath),
					zap.String("root", absRoot),
				)
				return nil
			}
		}

		// 파일 읽기
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			errs = append(errs, fmt.Errorf("파일 읽기 실패 %s: %w", path, readErr))
			return nil
		}

		// 파싱
		def, parseErr := ParseSkillFile(path, content)
		if parseErr != nil {
			errs = append(errs, fmt.Errorf("파싱 실패 %s: %w", path, parseErr))
			options.logger.Warn("skill 파싱 실패 — 건너뜀",
				zap.String("path", path),
				zap.Error(parseErr),
			)
			return nil
		}

		// 중복 ID 검사 (REQ-SK-004)
		if firstPath, exists := firstRegistered[def.ID]; exists {
			errs = append(errs, ErrDuplicateSkillID{
				ID:   def.ID,
				Path: path,
			})
			options.logger.Warn("중복 skill ID 감지 — 건너뜀",
				zap.String("skill_id", def.ID),
				zap.String("first_path", firstPath),
				zap.String("duplicate_path", path),
			)
			return nil
		}

		// 레지스트리에 등록
		reg.mu.Lock()
		reg.skills[def.ID] = def
		reg.mu.Unlock()

		firstRegistered[def.ID] = path
		options.logger.Info("skill 로드 완료",
			zap.String("skill_id", def.ID),
			zap.String("trigger", triggerName(def.Trigger)),
			zap.Int("effort_level", int(def.Effort)),
		)

		return nil
	})

	if walkErr != nil {
		errs = append(errs, fmt.Errorf("디렉토리 walk 실패: %w", walkErr))
	}

	return reg, errs
}

// isSymlink는 파일이 symlink인지 확인한다 (Lstat 기반).
func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// triggerName은 TriggerMode를 로깅용 문자열로 변환한다.
func triggerName(t TriggerMode) string {
	switch t {
	case TriggerInline:
		return "inline"
	case TriggerFork:
		return "fork"
	case TriggerConditional:
		return "conditional"
	case TriggerRemote:
		return "remote"
	default:
		return "unknown"
	}
}
