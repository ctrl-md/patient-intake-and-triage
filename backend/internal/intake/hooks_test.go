package intake

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func hasWord(texts []string, w string) bool {
	for _, t := range texts {
		if strings.Contains(strings.ToLower(t), w) {
			return true
		}
	}
	return false
}

func hookedEnv(t *testing.T) (*env, *[]string, *int) {
	e := newEnv(t)
	var order []string
	triaged := 0
	e.h.background = func(f func()) { f() } // run inline in tests
	e.h.WithHooks(Hooks{
		Emergency: func(texts []string) bool {
			order = append(order, "rules")
			return hasWord(texts, "chest pain")
		},
		EmergencyReply: "CALL EMERGENCY",
		Reply: func(_ context.Context, h []Message) string {
			order = append(order, "ai")
			return "How long?"
		},
		Triage: func(_ context.Context, id string, h []Message) { triaged++ },
	})
	return e, &order, &triaged
}

func TestReplyAddedAfterPatientMessage(t *testing.T) {
	e, order, _ := hookedEnv(t)
	id, token := e.newIntake(t, "")
	rec := e.do("POST", publicPath(token, "/messages"), `{"content":"I have a headache"}`, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	m := jsonBody(t, rec)
	if m["reply"].(map[string]any)["content"] != "How long?" || m["emergency"] != false {
		t.Fatalf("bad body: %v", m)
	}
	if got := strings.Join(*order, ","); got != "rules,ai" {
		t.Fatalf("rules must run before the AI, got %s", got)
	}
	msgs, _ := e.store.Messages(context.Background(), id)
	if len(msgs) != 2 || msgs[1].Role != RoleAssistant {
		t.Fatalf("messages: %+v", msgs)
	}
}

func TestEmergencySkipsAI(t *testing.T) {
	e, order, _ := hookedEnv(t)
	_, token := e.newIntake(t, "")
	rec := e.do("POST", publicPath(token, "/messages"), `{"content":"I have chest pain"}`, "")
	m := jsonBody(t, rec)
	if m["emergency"] != true || m["reply"].(map[string]any)["content"] != "CALL EMERGENCY" {
		t.Fatalf("bad body: %v", m)
	}
	if strings.Contains(strings.Join(*order, ","), "ai") {
		t.Fatalf("AI must not run on an emergency: %v", *order)
	}
}

func TestNoHooksKeepsOldBehaviour(t *testing.T) {
	e := newEnv(t)
	_, token := e.newIntake(t, "")
	m := jsonBody(t, e.do("POST", publicPath(token, "/messages"), `{"content":"hi"}`, ""))
	if _, has := m["reply"]; has {
		t.Fatal("no reply expected without hooks")
	}
}

func TestEmergencyShownEvenWhenMessageLimitHit(t *testing.T) {
	e, _, _ := hookedEnv(t)
	e.h.maxMessages = 1
	_, token := e.newIntake(t, "")
	m := jsonBody(t, e.do("POST", publicPath(token, "/messages"), `{"content":"chest pain"}`, ""))
	if m["emergency"] != true {
		t.Fatalf("emergency warning must still be shown: %v", m)
	}
}

func TestTriageRunsOnComplete(t *testing.T) {
	e, _, triaged := hookedEnv(t)
	_, token := e.newIntake(t, "")
	e.do("POST", publicPath(token, "/messages"), `{"content":"cough"}`, "")
	if rec := e.do("POST", publicPath(token, "/complete"), "", ""); rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if *triaged != 1 {
		t.Fatalf("triage ran %d times", *triaged)
	}
	// calling complete again must not triage again
	e.do("POST", publicPath(token, "/complete"), "", "")
	if *triaged != 1 {
		t.Fatalf("triage ran again: %d", *triaged)
	}
}

func TestListShowsUrgency(t *testing.T) {
	e := newEnv(t)
	id, _ := e.newIntake(t, "")
	u := "urgent"
	e.store.intakes[id].Urgency = &u
	m := jsonBody(t, e.staffDo("GET", "/intakes", ""))
	first := m["intakes"].([]any)[0].(map[string]any)
	if first["urgency"] != "urgent" {
		t.Fatalf("urgency: %v", first["urgency"])
	}
}
