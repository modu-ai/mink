<!--
PR template for MINK (modu-ai/mink).
Reference: CLAUDE.local.md §2.3 PR 생성 규약.
- title: 70자 이내 영문 conventional + 한국어 설명
- body: 한국어
- merge: feature/* → squash, release/* → merge commit, hotfix/* → squash
- 필수 label: type/* + priority/* (area/* 권장)
-->

## Summary
<!-- 1-3 bullets: 왜 / 무엇이 바뀌나 -->

-

## Context
<!-- 배경, SPEC 참조, 관련 이슈 번호 -->

- SPEC: <!-- SPEC-GOOSE-XXX-YYY 또는 N/A -->
- Related: <!-- #issue 또는 #PR 또는 N/A -->

## Changes
<!-- 주요 변경 사항을 영역별로 정리 -->

-

## Test Plan
<!-- 검증 방법 + 결과. CI status check가 자동 검증하지만 사람이 알아야 할 내용 정리 -->

- [ ] `go test -race ./...` 통과
- [ ] `go vet ./...` 통과
- [ ] `gofmt -l .` 비어 있음
- [ ] `bash scripts/check-brand.sh` 통과
- [ ] 신규/수정된 패키지 커버리지 ≥85%
- [ ] 신규 코드 주석 영어로 작성 (CLAUDE.local.md §2.5)
- [ ] @MX 태그 추가/갱신 (해당 시)

## Coverage
<!-- 변경된 패키지의 before / after 커버리지 -->

| Package | Before | After |
|---------|-------:|------:|
|         |        |       |

## Design Decisions
<!-- 비자명한 설계 선택, 트레이드오프, 대안 거절 이유 -->

-

## Review Focus
<!-- 리뷰어가 특별히 봐야 할 부분 -->

-

## Migration / Compatibility
<!-- 깨지는 변경(breaking change), 마이그레이션 단계가 있다면 명시 -->

- [ ] Breaking change 없음
- [ ] 또는 migration 노트:

## Checklist
- [ ] type/* label 부여
- [ ] priority/* label 부여
- [ ] area/* label 부여(해당 시)
- [ ] CHANGELOG는 release-drafter가 자동 처리 — 수동 편집 불필요
- [ ] CLAUDE.local.md §1.4 merge 전략 확인 완료
