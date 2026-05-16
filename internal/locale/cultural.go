package locale

// culturalEntry holds the static cultural metadata for a given ISO 3166-1 alpha-2
// country code. This table is the sole source of truth for CulturalContext derivation.
//
// Extension policy: add entries as PRs with CLDR 44.1 source citations in the commit body.
// @MX:NOTE: [AUTO] CLDR 44.1 (2024-04) is the data source for currency, timezone, and cultural mappings.
type culturalEntry struct {
	formality      FormalityMode
	honorific      string
	nameOrder      string
	addressFormat  string
	weekendDays    []string
	firstDayOfWeek string
	legalFlags     []string
	calendarSystem string // overrides default "gregorian"
}

// countryToCultural is the static mapping from ISO 3166-1 alpha-2 → cultural metadata.
// Covers ~20 priority countries per SPEC §6.3. Unlisted countries fall back to the
// defaultCultural value.
var countryToCultural = map[string]culturalEntry{
	"KR": {
		formality:      FormalityFormal,
		honorific:      "korean_jondaetmal",
		nameOrder:      "family_first",
		addressFormat:  "east_asian",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"pipa"},
		calendarSystem: "gregorian",
	},
	"JP": {
		formality:      FormalityFormal,
		honorific:      "japanese_keigo",
		nameOrder:      "family_first",
		addressFormat:  "east_asian",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"appi"},
		calendarSystem: "gregorian",
	},
	"CN": {
		formality:      FormalityFormal,
		honorific:      "chinese_jing",
		nameOrder:      "family_first",
		addressFormat:  "east_asian",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"pipl"},
		calendarSystem: "gregorian",
	},
	"US": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"ccpa"},
		calendarSystem: "gregorian",
	},
	"DE": {
		formality:      FormalityFormal,
		honorific:      "german_sie_du",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	"FR": {
		formality:      FormalityFormal,
		honorific:      "french_tu_vous",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	"GB": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"ukgdpr"},
		calendarSystem: "gregorian",
	},
	"BR": {
		formality:      FormalityFormal,
		honorific:      "portuguese_senhor",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"lgpd"},
		calendarSystem: "gregorian",
	},
	"RU": {
		formality:      FormalityFormal,
		honorific:      "russian_vy",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"fz152"},
		calendarSystem: "gregorian",
	},
	"SA": {
		formality:      FormalityFormal,
		honorific:      "arabic_formal_familiar",
		nameOrder:      "family_first",
		addressFormat:  "western",
		weekendDays:    []string{"Fri", "Sat"},
		firstDayOfWeek: "Saturday",
		legalFlags:     []string{"sa_pdpl"},
		calendarSystem: "hijri",
	},
	"AE": {
		formality:      FormalityFormal,
		honorific:      "arabic_formal_familiar",
		nameOrder:      "family_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Saturday",
		legalFlags:     []string{"uae_pdpl"},
		calendarSystem: "gregorian",
	},
	"IN": {
		formality:      FormalityFormal,
		honorific:      "hindi_aap_tum",
		nameOrder:      "family_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"in_dpdp"},
		calendarSystem: "gregorian",
	},
	"VN": {
		formality:      FormalityFormal,
		honorific:      "vietnamese_anh_em",
		nameOrder:      "family_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"vn_pdpd"},
		calendarSystem: "gregorian",
	},
	"ID": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"id_pdp"},
		calendarSystem: "gregorian",
	},
	"TH": {
		formality:      FormalityFormal,
		honorific:      "thai_khun",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"th_pdpa"},
		calendarSystem: "thai_buddhist",
	},
	"TR": {
		formality:      FormalityFormal,
		honorific:      "turkish_siz",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"kvkk"},
		calendarSystem: "gregorian",
	},
	"MX": {
		formality:      FormalityFormal,
		honorific:      "spanish_usted_tu",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"mx_lfpdppp"},
		calendarSystem: "gregorian",
	},
	"ES": {
		formality:      FormalityFormal,
		honorific:      "spanish_usted_tu",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	"IT": {
		formality:      FormalityFormal,
		honorific:      "italian_lei_tu",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	"PL": {
		formality:      FormalityFormal,
		honorific:      "polish_pan_pani",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	// Additional priority countries for currency/timezone coverage
	"AU": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"au_privacy"},
		calendarSystem: "gregorian",
	},
	"CA": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"pipeda"},
		calendarSystem: "gregorian",
	},
	"NL": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	"SE": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	"NO": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"gdpr"},
		calendarSystem: "gregorian",
	},
	"CH": {
		formality:      FormalityFormal,
		honorific:      "german_sie_du",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"ch_dsg"},
		calendarSystem: "gregorian",
	},
	"NZ": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"nz_ppa"},
		calendarSystem: "gregorian",
	},
	"SG": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"pdpa_sg"},
		calendarSystem: "gregorian",
	},
	"MY": {
		formality:      FormalityFormal,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Monday",
		legalFlags:     []string{"pdpa_my"},
		calendarSystem: "gregorian",
	},
	"PH": {
		formality:      FormalityFormal,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"ph_dpa"},
		calendarSystem: "gregorian",
	},
	"ZA": {
		formality:      FormalityCasual,
		honorific:      "none",
		nameOrder:      "given_first",
		addressFormat:  "western",
		weekendDays:    []string{"Sat", "Sun"},
		firstDayOfWeek: "Sunday",
		legalFlags:     []string{"popia"},
		calendarSystem: "gregorian",
	},
	"EG": {
		formality:      FormalityFormal,
		honorific:      "arabic_formal_familiar",
		nameOrder:      "family_first",
		addressFormat:  "western",
		weekendDays:    []string{"Fri", "Sat"},
		firstDayOfWeek: "Saturday",
		legalFlags:     []string{},
		calendarSystem: "gregorian",
	},
}

// defaultCultural is returned for any country not listed in countryToCultural.
// Defaults to US-style casual/none with no legal flags and gregorian calendar.
var defaultCultural = culturalEntry{
	formality:      FormalityCasual,
	honorific:      "none",
	nameOrder:      "given_first",
	addressFormat:  "western",
	weekendDays:    []string{"Sat", "Sun"},
	firstDayOfWeek: "Monday",
	legalFlags:     []string{},
	calendarSystem: "gregorian",
}

// countryToCurrency maps ISO 3166-1 alpha-2 → ISO 4217 currency code.
//
// Source: Unicode CLDR 44.1 supplemental/supplementalData.xml <currencyData>.
// Covers ~30 priority countries. Unknown countries fall back to "USD".
//
// @MX:NOTE: [AUTO] CLDR-inspired manual map per SPEC §6.8. Extension via PR with CLDR 44.1 citation.
var countryToCurrency = map[string]string{
	"US": "USD",
	"KR": "KRW",
	"JP": "JPY",
	"CN": "CNY",
	"GB": "GBP",
	"DE": "EUR",
	"FR": "EUR",
	"IT": "EUR",
	"ES": "EUR",
	"NL": "EUR",
	"BE": "EUR",
	"AT": "EUR",
	"PT": "EUR",
	"FI": "EUR",
	"GR": "EUR",
	"PL": "PLN",
	"CA": "CAD",
	"AU": "AUD",
	"NZ": "NZD",
	"CH": "CHF",
	"SE": "SEK",
	"NO": "NOK",
	"DK": "DKK",
	"BR": "BRL",
	"MX": "MXN",
	"AR": "ARS",
	"IN": "INR",
	"RU": "RUB",
	"ZA": "ZAR",
	"EG": "EGP",
	"SA": "SAR",
	"AE": "AED",
	"TR": "TRY",
	"ID": "IDR",
	"TH": "THB",
	"VN": "VND",
	"PH": "PHP",
	"SG": "SGD",
	"MY": "MYR",
	"HK": "HKD",
	"TW": "TWD",
	"IL": "ILS",
	"NG": "NGN",
	"KE": "KES",
	"PK": "PKR",
	"BD": "BDT",
}

// CountryToCurrency returns the ISO 4217 currency code for the given ISO 3166-1
// alpha-2 country code. The second return value is false when the country is not
// in the table and "USD" is returned as a fallback.
//
// @MX:ANCHOR: [AUTO] Primary currency lookup used by Detect() and cultural context derivation.
// @MX:REASON: Called by Detect() for every locale resolution; incorrect currency silently propagates.
func CountryToCurrency(country string) (string, bool) {
	if c, ok := countryToCurrency[country]; ok {
		return c, true
	}
	return "USD", false
}

// countryToTimezones maps ISO 3166-1 alpha-2 → ordered IANA timezone list.
//
// For multi-timezone countries the first element is the CLDR primary zone
// (likelySubtags). Single-timezone countries are omitted; use countryPrimaryTZ
// for the canonical lookup.
//
// @MX:NOTE: [AUTO] Multi-TZ policy per SPEC §6.9: OS TZ env > CLDR primary > timezone_alternatives.
var countryToTimezones = map[string][]string{
	"US": {
		"America/New_York",
		"America/Chicago",
		"America/Denver",
		"America/Los_Angeles",
		"America/Anchorage",
		"Pacific/Honolulu",
	},
	"RU": {
		"Europe/Moscow",
		"Europe/Kaliningrad",
		"Europe/Samara",
		"Asia/Yekaterinburg",
		"Asia/Omsk",
		"Asia/Krasnoyarsk",
		"Asia/Irkutsk",
		"Asia/Yakutsk",
		"Asia/Vladivostok",
		"Asia/Magadan",
		"Asia/Kamchatka",
	},
	"BR": {
		"America/Sao_Paulo",
		"America/Fortaleza",
		"America/Manaus",
		"America/Noronha",
	},
	"CA": {
		"America/Toronto",
		"America/Vancouver",
		"America/Edmonton",
		"America/Winnipeg",
		"America/Halifax",
		"America/St_Johns",
	},
	"AU": {
		"Australia/Sydney",
		"Australia/Melbourne",
		"Australia/Brisbane",
		"Australia/Perth",
		"Australia/Adelaide",
	},
}

// countryPrimaryTZ returns the CLDR primary IANA timezone for countries not in
// countryToTimezones (i.e., single-timezone countries). Values sourced from
// CLDR likelySubtags.xml.
var countryPrimaryTZ = map[string]string{
	"KR": "Asia/Seoul",
	"JP": "Asia/Tokyo",
	"CN": "Asia/Shanghai",
	"GB": "Europe/London",
	"DE": "Europe/Berlin",
	"FR": "Europe/Paris",
	"IT": "Europe/Rome",
	"ES": "Europe/Madrid",
	"NL": "Europe/Amsterdam",
	"SE": "Europe/Stockholm",
	"NO": "Europe/Oslo",
	"CH": "Europe/Zurich",
	"PL": "Europe/Warsaw",
	"TR": "Europe/Istanbul",
	"IN": "Asia/Kolkata",
	"SA": "Asia/Riyadh",
	"AE": "Asia/Dubai",
	"ID": "Asia/Jakarta",
	"TH": "Asia/Bangkok",
	"VN": "Asia/Ho_Chi_Minh",
	"PH": "Asia/Manila",
	"SG": "Asia/Singapore",
	"MY": "Asia/Kuala_Lumpur",
	"NZ": "Pacific/Auckland",
	"ZA": "Africa/Johannesburg",
	"EG": "Africa/Cairo",
	"MX": "America/Mexico_City",
	"AR": "America/Argentina/Buenos_Aires",
	"HK": "Asia/Hong_Kong",
	"TW": "Asia/Taipei",
	"IL": "Asia/Jerusalem",
}

// PrimaryTimezone returns the CLDR primary IANA timezone for the given country.
// For multi-timezone countries it returns the primary zone (first element).
// Returns (zone, true) when found, ("UTC", false) when the country is unknown.
//
// @MX:ANCHOR: [AUTO] Called by Detect() as TZ fallback for countries with known primary zones.
// @MX:REASON: Incorrect default TZ propagates to scheduling (SCHEDULER-001) and cultural context.
func PrimaryTimezone(country string) (string, bool) {
	if zones, ok := countryToTimezones[country]; ok && len(zones) > 0 {
		return zones[0], true
	}
	if tz, ok := countryPrimaryTZ[country]; ok {
		return tz, true
	}
	return "UTC", false
}

// TimezoneAlternatives returns all known IANA timezone zones for multi-timezone
// countries. Returns nil for single-timezone countries or unknown countries.
func TimezoneAlternatives(country string) []string {
	if zones, ok := countryToTimezones[country]; ok && len(zones) > 1 {
		out := make([]string, len(zones))
		copy(out, zones)
		return out
	}
	return nil
}

// ResolveCulturalContext returns the CulturalContext for the given ISO 3166-1
// alpha-2 country code. The mapping is deterministic: identical inputs always
// produce identical outputs (REQ-LC-003, AC-LC-014).
//
// Unknown country codes fall back to a US-style default (casual, no honorifics,
// gregorian calendar, no legal flags).
//
// @MX:ANCHOR: [AUTO] Primary cultural context resolver — called by Detect() and LLM prompt builder.
// @MX:REASON: Returns static data; callers cache the result; any behavioral change requires all callers to re-test.
func ResolveCulturalContext(country string) CulturalContext {
	e, ok := countryToCultural[country]
	if !ok {
		e = defaultCultural
	}

	// Determine calendar system: prefer explicit entry; default to "gregorian".
	cal := e.calendarSystem
	if cal == "" {
		cal = "gregorian"
	}

	// Copy slices to prevent aliasing (callers may modify their copy).
	weekendCopy := make([]string, len(e.weekendDays))
	copy(weekendCopy, e.weekendDays)

	legalCopy := make([]string, len(e.legalFlags))
	copy(legalCopy, e.legalFlags)

	return CulturalContext{
		FormalityDefault: e.formality,
		HonorificSystem:  e.honorific,
		NameOrder:        e.nameOrder,
		AddressFormat:    e.addressFormat,
		WeekendDays:      weekendCopy,
		FirstDayOfWeek:   e.firstDayOfWeek,
		LegalFlags:       legalCopy,
	}
}

// detectCalendarSystem returns the primary calendar system for a country/language pair.
// Phase 1 returns "gregorian" for all inputs with a TODO for hijri/hebrew/persian overrides.
//
// TODO: Phase 2 — return "hijri" for SA/YE/AF/IR, "thai_buddhist" for TH, "hebrew" for IL.
func detectCalendarSystem(country, _ string) string {
	if e, ok := countryToCultural[country]; ok && e.calendarSystem != "" {
		return e.calendarSystem
	}
	return "gregorian"
}

// detectMeasurementSystem returns "imperial" for US, LR, MM; "metric" for all others.
// GB uses metric for most purposes (road speed uses mph but that is I18N-001's concern).
func detectMeasurementSystem(country string) string {
	switch country {
	case "US", "LR", "MM":
		return "imperial"
	default:
		return "metric"
	}
}
