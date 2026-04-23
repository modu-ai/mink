// Package credential_test는 선택 전략 테스트를 포함한다.
package credential_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/credential"
)

// TestRoundRobinStrategy_RotatesEvenly는 RoundRobin 전략이 N개의 크레덴셜에 걸쳐
// 균등하게 회전하는지 검증한다.
// AC-3: RoundRobin은 N개 크레덴셜에 걸쳐 N*k 번의 Select에서 균등하게 회전
func TestRoundRobinStrategy_RotatesEvenly(t *testing.T) {
	t.Parallel()

	const n = 4
	const k = 5 // 각 크레덴셜이 k번 선택되어야 함

	creds := make([]*credential.PooledCredential, n)
	for i := range n {
		creds[i] = &credential.PooledCredential{
			ID:     string(rune('A' + i)),
			Status: credential.CredOK,
		}
	}

	strategy := credential.NewRoundRobinStrategy()
	counts := make(map[string]int)

	for range n * k {
		selected, err := strategy.Select(creds)
		if err != nil {
			t.Fatalf("Select 실패: %v", err)
		}
		counts[selected.ID]++
	}

	for _, c := range creds {
		if counts[c.ID] != k {
			t.Errorf("크레덴셜 %s: %d번 선택됨, %d번 기대", c.ID, counts[c.ID], k)
		}
	}
}

// TestRoundRobinStrategy_Name은 전략 이름을 올바르게 반환하는지 검증한다.
func TestRoundRobinStrategy_Name(t *testing.T) {
	t.Parallel()

	s := credential.NewRoundRobinStrategy()
	if s.Name() == "" {
		t.Error("전략 이름이 비어 있음")
	}
}

// TestRoundRobinStrategy_EmptyList_ReturnsError는 빈 리스트에서
// 에러를 반환하는지 검증한다.
func TestRoundRobinStrategy_EmptyList_ReturnsError(t *testing.T) {
	t.Parallel()

	s := credential.NewRoundRobinStrategy()
	_, err := s.Select(nil)
	if err == nil {
		t.Error("빈 리스트에서 에러 기대")
	}
}

// TestLRUStrategy_SelectsLeastRecentlyUsed는 LRU 전략이 UsageCount가
// 가장 낮은 크레덴셜을 선택하는지 검증한다.
func TestLRUStrategy_SelectsLeastRecentlyUsed(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "heavy", Status: credential.CredOK, UsageCount: 100},
		{ID: "light", Status: credential.CredOK, UsageCount: 5},
		{ID: "medium", Status: credential.CredOK, UsageCount: 50},
	}

	s := credential.NewLRUStrategy()
	selected, err := s.Select(creds)
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	if selected.ID != "light" {
		t.Errorf("LRU는 UsageCount가 가장 낮은 'light' 선택해야 함, got %s", selected.ID)
	}
}

// TestLRUStrategy_Name은 전략 이름을 올바르게 반환하는지 검증한다.
func TestLRUStrategy_Name(t *testing.T) {
	t.Parallel()

	s := credential.NewLRUStrategy()
	if s.Name() == "" {
		t.Error("전략 이름이 비어 있음")
	}
}

// TestWeightedStrategy_DistributionMatchesWeights는 Weighted 전략이
// Weight 비율에 맞게 크레덴셜을 선택하는지 검증한다.
// AC-10: WeightedStrategy는 10k Selects에서 카이제곱 검정 통과
func TestWeightedStrategy_DistributionMatchesWeights(t *testing.T) {
	t.Parallel()

	const trials = 10_000

	creds := []*credential.PooledCredential{
		{ID: "w1", Status: credential.CredOK, Weight: 1},
		{ID: "w2", Status: credential.CredOK, Weight: 2},
		{ID: "w7", Status: credential.CredOK, Weight: 7},
	}

	s := credential.NewWeightedStrategy()
	counts := make(map[string]int)

	for range trials {
		c, err := s.Select(creds)
		if err != nil {
			t.Fatalf("Select 실패: %v", err)
		}
		counts[c.ID]++
	}

	totalWeight := 0
	for _, c := range creds {
		totalWeight += c.Weight
	}

	// 카이제곱 검정 (유의수준 0.001, 자유도 2, chi2 임계값 ≈ 13.816)
	chi2 := 0.0
	for _, c := range creds {
		expected := float64(trials) * float64(c.Weight) / float64(totalWeight)
		observed := float64(counts[c.ID])
		diff := observed - expected
		chi2 += (diff * diff) / expected
	}

	// chi2 임계값 13.816 (자유도 2, p=0.001)
	const chi2Threshold = 13.816
	if chi2 > chi2Threshold {
		t.Errorf("Weighted 분포 카이제곱 검정 실패: chi2=%.4f > %.4f (임계값)", chi2, chi2Threshold)
		for _, c := range creds {
			expected := float64(trials) * float64(c.Weight) / float64(totalWeight)
			t.Logf("  %s: expected=%.0f, observed=%d", c.ID, expected, counts[c.ID])
		}
	}
}

// TestWeightedStrategy_ZeroWeight_Excluded는 Weight=0인 크레덴셜이
// 선택되지 않는지 검증한다.
func TestWeightedStrategy_ZeroWeight_Excluded(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "zero", Status: credential.CredOK, Weight: 0},
		{ID: "nonzero", Status: credential.CredOK, Weight: 5},
	}

	s := credential.NewWeightedStrategy()
	for range 100 {
		c, err := s.Select(creds)
		if err != nil {
			t.Fatalf("Select 실패: %v", err)
		}
		if c.ID == "zero" {
			t.Error("Weight=0인 크레덴셜이 선택됨")
		}
	}
}

// TestWeightedStrategy_Name은 전략 이름을 올바르게 반환하는지 검증한다.
func TestWeightedStrategy_Name(t *testing.T) {
	t.Parallel()

	s := credential.NewWeightedStrategy()
	if s.Name() == "" {
		t.Error("전략 이름이 비어 있음")
	}
}

// TestPriorityStrategy_SelectsHighestPriority는 Priority 전략이
// Priority 값이 가장 높은 크레덴셜을 선택하는지 검증한다.
func TestPriorityStrategy_SelectsHighestPriority(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "low", Status: credential.CredOK, Priority: 1},
		{ID: "high", Status: credential.CredOK, Priority: 10},
		{ID: "mid", Status: credential.CredOK, Priority: 5},
	}

	s := credential.NewPriorityStrategy()
	selected, err := s.Select(creds)
	if err != nil {
		t.Fatalf("Select 실패: %v", err)
	}
	if selected.ID != "high" {
		t.Errorf("Priority 전략은 가장 높은 우선순위 크레덴셜 선택해야 함, got %s", selected.ID)
	}
}

// TestPriorityStrategy_TiebreakRoundRobin은 동일 Priority일 때
// 라운드로빈으로 타이브레이크되는지 검증한다.
func TestPriorityStrategy_TiebreakRoundRobin(t *testing.T) {
	t.Parallel()

	creds := []*credential.PooledCredential{
		{ID: "a", Status: credential.CredOK, Priority: 5},
		{ID: "b", Status: credential.CredOK, Priority: 5},
	}

	s := credential.NewPriorityStrategy()
	counts := make(map[string]int)
	const trials = 10

	for range trials {
		c, err := s.Select(creds)
		if err != nil {
			t.Fatalf("Select 실패: %v", err)
		}
		counts[c.ID]++
	}

	// 동일 Priority이므로 두 크레덴셜 모두 선택되어야 함
	if counts["a"] == 0 || counts["b"] == 0 {
		t.Errorf("동일 Priority 타이브레이크 실패: a=%d, b=%d", counts["a"], counts["b"])
	}
}

// TestPriorityStrategy_Name은 전략 이름을 올바르게 반환하는지 검증한다.
func TestPriorityStrategy_Name(t *testing.T) {
	t.Parallel()

	s := credential.NewPriorityStrategy()
	if s.Name() == "" {
		t.Error("전략 이름이 비어 있음")
	}
}

// TestAllStrategies_EmptyList_ReturnsError는 모든 전략이 빈 리스트에서
// 에러를 반환하는지 검증한다.
func TestAllStrategies_EmptyList_ReturnsError(t *testing.T) {
	t.Parallel()

	strategies := []credential.SelectionStrategy{
		credential.NewRoundRobinStrategy(),
		credential.NewLRUStrategy(),
		credential.NewWeightedStrategy(),
		credential.NewPriorityStrategy(),
	}

	for _, s := range strategies {
		s := s
		t.Run(s.Name(), func(t *testing.T) {
			t.Parallel()
			_, err := s.Select(nil)
			if err == nil {
				t.Errorf("%s: 빈 리스트에서 에러 기대", s.Name())
			}
		})
	}
}
