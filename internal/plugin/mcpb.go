package plugin

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// varPattern은 ${VAR} 치환 패턴이다.
var varPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}`)

// extractMCPB는 .mcpb (zip) 파일을 임시 디렉토리에 압축 해제한다.
// REQ-PL-006, REQ-PL-014 (zip slip 방지)
//
// @MX:ANCHOR: [AUTO] extractMCPB — MCPB zip 해제의 단일 진입점
// @MX:REASON: REQ-PL-014 zip slip 방지 — 경로 탈출 검증이 이 함수에 집중됨. fan_in >= 3 (loader, test)
// @MX:SPEC: REQ-PL-006, REQ-PL-014
func extractMCPB(mcpbPath string) (string, error) {
	r, err := zip.OpenReader(mcpbPath)
	if err != nil {
		return "", fmt.Errorf("MCPB zip 열기 실패: %w", err)
	}
	defer r.Close()

	tmpDir, err := os.MkdirTemp("", "goose-mcpb-*")
	if err != nil {
		return "", fmt.Errorf("임시 디렉토리 생성 실패: %w", err)
	}

	for _, f := range r.File {
		targetPath := filepath.Join(tmpDir, filepath.Clean(f.Name))

		// zip slip 방지 (REQ-PL-014)
		if !strings.HasPrefix(targetPath, tmpDir+string(os.PathSeparator)) {
			os.RemoveAll(tmpDir) //nolint:errcheck
			return "", ErrZipSlip{Path: f.Name}
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				os.RemoveAll(tmpDir) //nolint:errcheck
				return "", fmt.Errorf("디렉토리 생성 실패: %w", err)
			}
			continue
		}

		// 상위 디렉토리 생성
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			os.RemoveAll(tmpDir) //nolint:errcheck
			return "", fmt.Errorf("상위 디렉토리 생성 실패: %w", err)
		}

		if err := extractZipFile(f, targetPath); err != nil {
			os.RemoveAll(tmpDir) //nolint:errcheck
			return "", err
		}
	}

	return tmpDir, nil
}

// extractZipFile은 단일 zip.File을 대상 경로에 추출한다.
func extractZipFile(f *zip.File, targetPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("zip 항목 열기 실패 %s: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("파일 생성 실패 %s: %w", targetPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("파일 복사 실패 %s: %w", targetPath, err)
	}
	return nil
}

// parseDXTManifest는 임시 디렉토리의 dxt-manifest.json을 읽어 DXTManifest를 반환한다.
func parseDXTManifest(tmpDir string) (*DXTManifest, error) {
	dxtPath := filepath.Join(tmpDir, "dxt-manifest.json")
	data, err := os.ReadFile(dxtPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DXTManifest{}, nil // dxt-manifest.json 없으면 빈 설정
		}
		return nil, fmt.Errorf("dxt-manifest.json 읽기 실패: %w", err)
	}

	var dxt DXTManifest
	if err := json.Unmarshal(data, &dxt); err != nil {
		return nil, fmt.Errorf("dxt-manifest.json 파싱 실패: %w", err)
	}
	return &dxt, nil
}

// applyUserConfigVars는 임시 디렉토리의 텍스트 파일들에서 ${VAR} 패턴을 치환한다.
// REQ-PL-006 step (c), REQ-PL-009: required 변수 누락 시 ErrMissingUserConfigVariable.
//
// @MX:WARN: [AUTO] 텍스트 파일 전체 치환 — 바이너리 파일은 건드리지 않음
// @MX:REASON: R3 리스크 — 치환 대상은 manifest.json, *.md, *.yaml 만; 바이너리 오염 방지
func applyUserConfigVars(tmpDir string, dxt *DXTManifest, vars map[string]string) error {
	if len(dxt.UserConfigVariables) == 0 {
		return nil
	}

	// 변수 값 맵 구성
	resolved := make(map[string]string)
	for _, ucv := range dxt.UserConfigVariables {
		val, ok := vars[ucv.Name]
		if !ok {
			if ucv.Default != nil {
				val = *ucv.Default
			} else if ucv.Required {
				return ErrMissingUserConfigVariable{Name: ucv.Name}
			}
		}
		resolved[ucv.Name] = val
	}

	// 텍스트 파일들에 치환 적용
	return filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// 치환 대상: .json, .md, .yaml, .yml
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".json" && ext != ".md" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		return substituteVarsInFile(path, resolved)
	})
}

// substituteVarsInFile은 파일 내 ${VAR} 패턴을 resolved 맵의 값으로 치환한다.
func substituteVarsInFile(path string, resolved map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("파일 읽기 실패 %s: %w", path, err)
	}

	replaced := varPattern.ReplaceAllStringFunc(string(data), func(match string) string {
		sub := varPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		if val, ok := resolved[sub[1]]; ok {
			return val
		}
		return match
	})

	return os.WriteFile(path, []byte(replaced), 0644)
}
