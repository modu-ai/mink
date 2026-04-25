package hook

// ScrubEnvForTest는 테스트에서 scrubEnv를 직접 호출할 수 있도록 export한다.
// AC-HK-021 / TestScrubEnv_DenyList
func ScrubEnvForTest(env []string) []string {
	return scrubEnv(env)
}
