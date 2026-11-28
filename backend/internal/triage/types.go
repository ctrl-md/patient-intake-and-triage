// Package triage decides how urgent an intake is.
//
// Order of work (safety first):
//  1. The rules engine reads what the patient wrote. It is plain Go, no AI.
//  2. The AI writes a summary and suggests an urgency.
//  3. The final urgency is the HIGHER of the two. The AI can raise it, never lower it.
//  4. If the AI fails, the case is bumped up to at least "urgent".
//
// This is a demo and not a medical device.
package triage

import "time"

type Urgency string

const (
	Routine   Urgency = "routine"
	Urgent    Urgency = "urgent"
	Emergency Urgency = "emergency"
)

func (u Urgency) Valid() bool { return u == Routine || u == Urgent || u == Emergency }

// Rank is used to compare. Bigger means more urgent.
func (u Urgency) Rank() int {
	switch u {
	case Emergency:
		return 3
	case Urgent:
		return 2
	default:
		return 1
	}
}

// Higher returns the more urgent of two.
func Higher(a, b Urgency) Urgency {
	if b.Rank() > a.Rank() {
		return b
	}
	return a
}

type RedFlag struct {
	Code     string  `json:"code"`
	Label    string  `json:"label"`
	Urgency  Urgency `json:"urgency"`
	Evidence string  `json:"evidence"` // the words the patient used
}

// Result is what we save for one intake.
type Result struct {
	IntakeID     string     `json:"intake_id"`
	RulesUrgency Urgency    `json:"rules_urgency"`
	RedFlags     []RedFlag  `json:"red_flags"`
	AIUrgency    *Urgency   `json:"ai_urgency"`   // nil when the AI failed
	AISummary    *string    `json:"ai_summary"`   //
	AIReasoning  *string    `json:"ai_reasoning"` //
	FinalUrgency Urgency    `json:"final_urgency"`
	StaffUrgency *Urgency   `json:"staff_urgency"`
	Override     *string    `json:"override_reason"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Combine applies the safety rules. ai is nil when the AI failed.
func Combine(rules Urgency, ai *Urgency) Urgency {
	if ai == nil {
		return Higher(rules, Urgent)
	}
	return Higher(rules, *ai)
}
