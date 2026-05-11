package journal

// emotionEntry holds the lexicon data for a single emotion category.
// Each entry provides Korean keywords, associated emoji, and a canonical VAD triple.
// research.md §2.2
type emotionEntry struct {
	// Keywords are Korean word stems/fragments matched via substring search.
	Keywords []string
	// Emoji are the Unicode emoji associated with this emotion.
	Emoji []string
	// Vad is the canonical VAD triple for this emotion category.
	Vad Vad
}

// emotionDict is the hardcoded Korean emotion dictionary.
// 12 categories: the 8 from research.md §2.2 plus 4 additional (lonely, regret, bored, proud).
// This map is the single source of truth; testdata/journal/emotion_dict.golden.yaml mirrors it.
var emotionDict = map[string]emotionEntry{
	"happy": {
		Keywords: []string{"행복", "기쁘", "좋아", "웃", "즐거", "신나", "설레", "만족", "뿌듯"},
		Emoji:    []string{"😊", "😄", "🥰", "😁", "🎉"},
		Vad:      Vad{Valence: 0.9, Arousal: 0.7, Dominance: 0.7},
	},
	"sad": {
		Keywords: []string{"슬프", "힘들", "외로", "울었", "눈물", "쓸쓸", "허전", "공허"},
		Emoji:    []string{"😢", "😭", "😔", "😞", "💔"},
		Vad:      Vad{Valence: 0.15, Arousal: 0.3, Dominance: 0.3},
	},
	"anxious": {
		Keywords: []string{"불안", "걱정", "초조", "긴장", "떨리", "걱정돼", "안절"},
		Emoji:    []string{"😰", "😟", "😨"},
		Vad:      Vad{Valence: 0.3, Arousal: 0.75, Dominance: 0.25},
	},
	"angry": {
		Keywords: []string{"화", "짜증", "열받", "빡", "분노", "억울", "답답"},
		Emoji:    []string{"😠", "😡", "🤬", "💢"},
		Vad:      Vad{Valence: 0.2, Arousal: 0.85, Dominance: 0.6},
	},
	"tired": {
		Keywords: []string{"피곤", "지쳐", "졸리", "나른", "녹초"},
		Emoji:    []string{"😴", "🥱", "😪"},
		Vad:      Vad{Valence: 0.4, Arousal: 0.15, Dominance: 0.35},
	},
	"calm": {
		Keywords: []string{"평온", "차분", "느긋", "조용", "고요", "편안", "안정"},
		Emoji:    []string{"😌", "🧘", "🌸"},
		Vad:      Vad{Valence: 0.65, Arousal: 0.25, Dominance: 0.6},
	},
	"excited": {
		Keywords: []string{"설렘", "기대", "두근", "흥분", "짜릿"},
		Emoji:    []string{"🤩", "🥳", "🎊"},
		Vad:      Vad{Valence: 0.85, Arousal: 0.9, Dominance: 0.7},
	},
	"grateful": {
		Keywords: []string{"감사", "고마", "감동", "뭉클", "따뜻"},
		Emoji:    []string{"🙏", "🥹", "❤️"},
		Vad:      Vad{Valence: 0.88, Arousal: 0.5, Dominance: 0.5},
	},
	"lonely": {
		Keywords: []string{"외롭", "혼자", "고독", "고립"},
		Emoji:    []string{"🥺", "😶"},
		Vad:      Vad{Valence: 0.2, Arousal: 0.25, Dominance: 0.2},
	},
	"regret": {
		Keywords: []string{"후회", "아쉬", "미안", "잘못"},
		Emoji:    []string{"😣", "😔"},
		Vad:      Vad{Valence: 0.25, Arousal: 0.4, Dominance: 0.3},
	},
	"bored": {
		Keywords: []string{"지루", "심심", "무료", "따분"},
		Emoji:    []string{"😑", "🙄"},
		Vad:      Vad{Valence: 0.45, Arousal: 0.1, Dominance: 0.4},
	},
	"proud": {
		Keywords: []string{"자랑", "성취", "해냈", "잘했", "대단"},
		Emoji:    []string{"💪", "🏆", "✨"},
		Vad:      Vad{Valence: 0.85, Arousal: 0.65, Dominance: 0.8},
	},
}

// negationTokens are Korean negation particles that flip the valence of a detected keyword.
// research.md §2.3
var negationTokens = []string{"안", "못", "없", "않"}

// intensityTokens are Korean intensifiers that boost the arousal of a detected keyword.
// research.md §2.4
var intensityTokens = []string{"너무", "정말", "엄청", "매우", "진짜"}
