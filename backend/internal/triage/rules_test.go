package triage

import "testing"

func TestEvaluateCases(t *testing.T) {
	cases := []struct {
		name string
		text string
		want Urgency
		code string // a code that must be present ("" = none needed)
	}{
		{"cold", "I have a runny nose and a mild cough since two days", Routine, ""},
		{"chest pain", "I have chest pain when I climb stairs", Emergency, "chest_pain"},
		{"chest pressure", "there is pressure in my chest", Emergency, "chest_pain"},
		{"cant breathe", "I can't breathe properly", Emergency, "breathing"},
		{"curly quote", "I can’t breathe", Emergency, "breathing"},
		{"stroke", "my mother has slurred speech and her face is drooping", Emergency, "stroke"},
		{"suicide", "I want to die", Emergency, "suicide"},
		{"seizure", "he had a seizure just now", Emergency, "seizure"},
		{"throat swelling", "my throat is swelling after eating peanuts", Emergency, "anaphylaxis"},
		{"fever high", "fever of 39.5 since last night", Urgent, "high_fever"},
		{"fever 103", "temperature 103 F", Urgent, "high_fever"},
		{"baby fever", "my 6 weeks old has a fever", Urgent, "infant_fever"},
		{"pain 10", "the pain is 10/10", Urgent, "severe_pain"},
		{"blood stool", "there is blood in my stool", Urgent, "blood_stool_urine"},
		{"preg bleeding", "I am pregnant and I am bleeding", Urgent, "pregnancy_bleeding"},
		{"worst headache", "this is the worst headache of my life", Emergency, "worst_headache"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, flags := Evaluate([]string{c.text})
			if got != c.want {
				t.Fatalf("urgency = %s, want %s (flags=%v)", got, c.want, flags)
			}
			if c.code != "" && !hasCode(flags, c.code) {
				t.Fatalf("missing flag %s in %v", c.code, flags)
			}
			if c.code == "" && len(flags) != 0 {
				t.Fatalf("unexpected flags %v", flags)
			}
		})
	}
}

func TestNegation(t *testing.T) {
	for _, text := range []string{
		"I have no chest pain",
		"I don't have chest pain, just a sore throat",
		"no shortness of breath and no fever",
		"I am not suicidal",
		"denies chest pain",
	} {
		if u, f := Evaluate([]string{text}); u != Routine {
			t.Errorf("%q should be routine, got %s %v", text, u, f)
		}
	}
}

func TestNegationDoesNotHideRealFlag(t *testing.T) {
	for _, text := range []string{
		"no one is home and I have chest pain",
		"I have no fever but I have chest pain now",
		"I did not sleep and my chest pain is getting worse",
		"I can't breathe",
		"I can't stand the chest pain",
		"I don't know what to do, my chest pain is bad",
	} {
		if u, _ := Evaluate([]string{text}); u != Emergency {
			t.Errorf("%q should be emergency, got %s", text, u)
		}
	}
}

func TestEvaluateUsesAllMessagesAndDedupes(t *testing.T) {
	u, flags := Evaluate([]string{"I feel dizzy", "now I have chest pain", "the chest pain is worse"})
	if u != Emergency || len(flags) != 1 {
		t.Fatalf("u=%s flags=%v", u, flags)
	}
	if flags[0].Evidence == "" {
		t.Fatal("evidence should not be empty")
	}
}

func TestEvaluateEmpty(t *testing.T) {
	if u, f := Evaluate(nil); u != Routine || f == nil || len(f) != 0 {
		t.Fatalf("u=%s f=%v", u, f)
	}
}

func TestEvidenceUTF8(t *testing.T) {
	u, f := Evaluate([]string{"ééééééééééééééé chest pain ééééééééééééééé"})
	if u != Emergency || len(f) != 1 {
		t.Fatal("expected a flag")
	}
	for _, r := range f[0].Evidence {
		if r == '�' {
			t.Fatal("broken UTF-8 in evidence")
		}
	}
}

func TestCombine(t *testing.T) {
	routine, urgent, emergency := Routine, Urgent, Emergency
	cases := []struct {
		rules Urgency
		ai    *Urgency
		want  Urgency
	}{
		{Routine, &routine, Routine},
		{Routine, &urgent, Urgent},
		{Routine, &emergency, Emergency},
		{Emergency, &routine, Emergency}, // AI cannot lower
		{Urgent, &routine, Urgent},
		{Routine, nil, Urgent}, // AI failed: bumped up
		{Urgent, nil, Urgent},
		{Emergency, nil, Emergency},
	}
	for _, c := range cases {
		if got := Combine(c.rules, c.ai); got != c.want {
			t.Errorf("Combine(%s, %v) = %s, want %s", c.rules, c.ai, got, c.want)
		}
	}
}

func hasCode(flags []RedFlag, code string) bool {
	for _, f := range flags {
		if f.Code == code {
			return true
		}
	}
	return false
}
