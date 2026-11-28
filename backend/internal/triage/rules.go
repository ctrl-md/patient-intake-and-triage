package triage

import (
	"regexp"
	"strings"
)

type rule struct {
	code    string
	label   string
	urgency Urgency
	re      *regexp.Regexp
}

func r(code, label string, u Urgency, pattern string) rule {
	return rule{code: code, label: label, urgency: u, re: regexp.MustCompile(`(?i)` + pattern)}
}

// The rule list. Add a rule by adding a line. Patterns are matched on lowercase
// text with curly quotes turned into plain ones.
var rules = []rule{
	// ----- emergency -----
	r("chest_pain", "Chest pain or pressure", Emergency,
		`chest (pain|pains|pressure|tightness|is tight|feels tight|heaviness)|(pain|pressure|tightness|heaviness|discomfort) (in|across) (my |the )?chest|tight chest|crushing`),
	r("breathing", "Severe trouble breathing", Emergency,
		`can'?t breathe|cannot breathe|unable to breathe|struggling to breathe|gasping|hard to breathe|difficulty breathing|trouble breathing|not breathing|stopped breathing|short(ness)? of breath|lips (are |turning )?blue`),
	r("stroke", "Possible stroke signs", Emergency,
		`face (is )?drooping|drooping face|slurred speech|slurring|sudden(ly)? (weak|numb)\w*|(weak|numb)\w* (on|in) (one|my left|my right) (side|arm|leg)|can'?t (move|lift) (my )?(arm|leg)|sudden (loss of )?vision|sudden confusion`),
	r("unconscious", "Passed out or unresponsive", Emergency,
		`unconscious|unresponsive|passed out|blacked out|collapsed|won'?t wake|not waking|fainted`),
	r("seizure", "Seizure", Emergency,
		`seizure|convuls|fitting|having a fit`),
	r("bleeding", "Severe bleeding", Emergency,
		`won'?t stop bleeding|bleeding (heavily|a lot|badly|won'?t stop)|heavy bleeding|severe bleeding|blood everywhere|losing a lot of blood`),
	r("blood_vomit_cough", "Vomiting or coughing up blood", Emergency,
		`(vomit\w*|throw\w* up|cough\w*|spit\w*) (up )?(a lot of )?blood|bloody vomit|blood in (my )?vomit`),
	r("suicide", "Thoughts of suicide or self-harm", Emergency,
		`suicid|kill myself|end my life|want to die|better off dead|hurt myself|harm myself|self[- ]harm`),
	r("anaphylaxis", "Possible severe allergic reaction", Emergency,
		`(throat|tongue|lips|face) (is |are |feels? |getting |started )?(swelling|swollen|closing|tight)|swelling (of|in) (my )?(throat|tongue|face)|anaphyla`),
	r("overdose", "Possible overdose or poisoning", Emergency,
		`overdos|took too many (pills|tablets)|swallowed (poison|bleach|chemicals?)|poisoned`),
	r("worst_headache", "Sudden severe headache", Emergency,
		`worst headache|thunderclap|sudden (severe |terrible )?headache`),
	r("head_injury", "Serious head injury", Emergency,
		`(hit|struck|injured) (my |his |her |the )?head.{0,40}(vomit|confus|drowsy|passed out|unconscious|can'?t remember)|head injury.{0,40}(vomit|confus|drowsy|unconscious)`),
	r("baby_breathing", "Baby not breathing or not responding", Emergency,
		`(baby|infant|newborn).{0,40}(not breathing|blue|limp|floppy|unresponsive|won'?t wake)`),

	// ----- urgent -----
	r("high_fever", "High fever", Urgent,
		`(fever|temperature).{0,20}(39|40|41|10[3-6])|(39|40|41|10[3-6]) ?(°|degrees|c\b|f\b)|very high fever|high fever`),
	r("infant_fever", "Fever in a baby", Urgent,
		`(baby|infant|newborn|\d+ ?(weeks?|months?) old).{0,40}fever|fever.{0,40}(baby|infant|newborn)`),
	r("severe_pain", "Severe pain", Urgent,
		`(severe|unbearable|excruciating|terrible|worst) (pain|ache)|pain (is )?(10|nine|ten|9) ?(/|out of)? ?10|10 ?/ ?10|9 ?/ ?10|screaming in pain`),
	r("abdominal_pain", "Severe belly pain", Urgent,
		`(severe|sharp|terrible|unbearable) (stomach|belly|abdominal|abdomen) pain|(stomach|belly|abdominal|abdomen) pain.{0,30}(severe|unbearable|worst)`),
	r("blood_stool_urine", "Blood in stool or urine", Urgent,
		`blood (in|when).{0,12}(stool|poo|urine|pee)|bloody (stool|poo|urine)|black (tarry )?stool|passing blood`),
	r("pregnancy_bleeding", "Pregnancy with bleeding or pain", Urgent,
		`pregnan\w*.{0,60}(bleeding|spotting|severe pain|cramp\w*|no movement|not moving)|(bleeding|severe pain|cramp\w*).{0,60}pregnan\w*`),
	r("dehydration", "Cannot keep fluids down", Urgent,
		`can'?t keep (anything|fluids|water) down|vomiting (for|since|all)|vomiting (a lot|constantly|nonstop)|dehydrat|not (peeing|urinating)`),
	r("fracture", "Possible broken bone", Urgent,
		`broken (bone|arm|leg|wrist|ankle|hip|rib)|(arm|leg|wrist|ankle|hip) (is )?(broken|deformed|bent)|bone (is )?sticking|heard a (snap|crack)`),
	r("vision_eye", "Eye injury or sudden vision change", Urgent,
		`(chemical|something) (in|splashed in) (my )?eye|lost (my )?vision|vision (loss|going)|eye injury`),
	r("diabetes_crisis", "Possible diabetes emergency", Urgent,
		`(diabet\w*|insulin|sugar).{0,40}(very high|very low|confus|shaky and sweaty|vomiting)`),
}

var (
	negationWords = regexp.MustCompile(`(?i)^(no|not|never|without|denies|denied|nor|free|negative|don't|doesn't|didn't|haven't|hasn't|isn't|wasn't|aren't)$`)
	wordPattern   = regexp.MustCompile(`[a-z0-9']+`)
	boundary      = regexp.MustCompile(`[.,;:!?\n]|\b(but|however|although|and)\b`)
)

// negated says whether the match is something the patient said they do NOT have.
// It looks at the three words just before the match, inside the same clause
// (a comma, "but" or "and" ends the clause).
// If we are not sure, we say false, so the flag is raised. That is the safe side.
func negated(text string, start int) bool {
	before := text[:start]
	if locs := boundary.FindAllStringIndex(before, -1); len(locs) > 0 {
		before = before[locs[len(locs)-1][1]:]
	}
	words := wordPattern.FindAllString(strings.ToLower(before), -1)
	if len(words) > 3 {
		words = words[len(words)-3:]
	}
	for _, w := range words {
		if negationWords.MatchString(w) {
			return true
		}
	}
	return false
}

func normalize(s string) string {
	s = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`).Replace(s)
	return strings.ToLower(s)
}

// Evaluate runs every rule over the patient's own messages (not the assistant's)
// and returns the highest urgency found and the flags that fired.
func Evaluate(patientTexts []string) (Urgency, []RedFlag) {
	text := normalize(strings.Join(patientTexts, "\n"))
	level := Routine
	flags := []RedFlag{}
	seen := map[string]bool{}

	for _, rl := range rules {
		for _, loc := range rl.re.FindAllStringIndex(text, -1) {
			if negated(text, loc[0]) {
				continue
			}
			if seen[rl.code] {
				break
			}
			seen[rl.code] = true
			flags = append(flags, RedFlag{
				Code: rl.code, Label: rl.label, Urgency: rl.urgency,
				Evidence: evidence(text, loc[0], loc[1]),
			})
			level = Higher(level, rl.urgency)
			break
		}
	}
	return level, flags
}

// evidence returns the matched words with a little context, kept short.
func evidence(text string, start, end int) string {
	from, to := start-15, end+15
	if from < 0 {
		from = 0
	}
	if to > len(text) {
		to = len(text)
	}
	// do not cut a UTF-8 character in half
	for from > 0 && !isRuneStart(text[from]) {
		from--
	}
	for to < len(text) && !isRuneStart(text[to]) {
		to++
	}
	return strings.TrimSpace(strings.Join(strings.Fields(text[from:to]), " "))
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
